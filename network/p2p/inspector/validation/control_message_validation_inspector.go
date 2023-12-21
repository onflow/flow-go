package validation

import (
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/common/worker"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/internal/cache"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/utils/logging"
	flowrand "github.com/onflow/flow-go/utils/rand"
)

// ControlMsgValidationInspector RPC message inspector that inspects control messages and performs some validation on them,
// when some validation rule is broken feedback is given via the Peer scoring notifier.
type ControlMsgValidationInspector struct {
	component.Component
	events.Noop
	ctx     irrecoverable.SignalerContext
	logger  zerolog.Logger
	sporkID flow.Identifier
	metrics module.GossipSubRpcValidationInspectorMetrics
	// config control message validation configurations.
	config *p2pconf.GossipSubRPCValidationInspectorConfigs
	// distributor used to disseminate invalid RPC message notifications.
	distributor p2p.GossipSubInspectorNotifDistributor
	// workerPool queue that stores *InspectRPCRequest that will be processed by component workers.
	workerPool *worker.Pool[*InspectRPCRequest]
	// tracker is a map that associates the hash of a peer's ID with the
	// number of cluster-prefix topic control messages received from that peer. It helps in tracking
	// and managing the rate of incoming control messages from each peer, ensuring that the system
	// stays performant and resilient against potential spam or abuse.
	// The counter is incremented in the following scenarios:
	// 1. The cluster prefix topic is received while the inspector waits for the cluster IDs provider to be set (this can happen during the startup or epoch transitions).
	// 2. The node sends a cluster prefix topic where the cluster prefix does not match any of the active cluster IDs.
	// In such cases, the inspector will allow a configured number of these messages from the corresponding peer.
	tracker      *cache.ClusterPrefixedMessagesReceivedTracker
	idProvider   module.IdentityProvider
	rateLimiters map[p2pmsg.ControlMessageType]p2p.BasicRateLimiter
	rpcTracker   p2p.RpcControlTracking
}

var _ component.Component = (*ControlMsgValidationInspector)(nil)
var _ p2p.GossipSubRPCInspector = (*ControlMsgValidationInspector)(nil)
var _ protocol.Consumer = (*ControlMsgValidationInspector)(nil)

// NewControlMsgValidationInspector returns new ControlMsgValidationInspector
// Args:
//   - logger: the logger used by the inspector.
//   - sporkID: the current spork ID.
//   - config: inspector configuration.
//   - distributor: gossipsub inspector notification distributor.
//   - clusterPrefixedCacheCollector: metrics collector for the underlying cluster prefix received tracker cache.
//   - idProvider: identity provider is used to get the flow identifier for a peer.
//
// Returns:
//   - *ControlMsgValidationInspector: a new control message validation inspector.
//   - error: an error if there is any error while creating the inspector. All errors are irrecoverable and unexpected.
func NewControlMsgValidationInspector(ctx irrecoverable.SignalerContext,
	logger zerolog.Logger,
	sporkID flow.Identifier,
	config *p2pconf.GossipSubRPCValidationInspectorConfigs,
	distributor p2p.GossipSubInspectorNotifDistributor,
	inspectMsgQueueCacheCollector module.HeroCacheMetrics,
	clusterPrefixedCacheCollector module.HeroCacheMetrics,
	idProvider module.IdentityProvider,
	inspectorMetrics module.GossipSubRpcValidationInspectorMetrics,
	rpcTracker p2p.RpcControlTracking) (*ControlMsgValidationInspector, error) {
	lg := logger.With().Str("component", "gossip_sub_rpc_validation_inspector").Logger()

	clusterPrefixedTracker, err := cache.NewClusterPrefixedMessagesReceivedTracker(logger,
		config.ClusterPrefixedControlMsgsReceivedCacheSize,
		clusterPrefixedCacheCollector,
		config.ClusterPrefixedControlMsgsReceivedCacheDecay)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster prefix topics received tracker")
	}

	c := &ControlMsgValidationInspector{
		ctx:          ctx,
		logger:       lg,
		sporkID:      sporkID,
		config:       config,
		distributor:  distributor,
		tracker:      clusterPrefixedTracker,
		rpcTracker:   rpcTracker,
		idProvider:   idProvider,
		metrics:      inspectorMetrics,
		rateLimiters: make(map[p2pmsg.ControlMessageType]p2p.BasicRateLimiter),
	}

	store := queue.NewHeroStore(config.CacheSize, logger, inspectMsgQueueCacheCollector)
	pool := worker.NewWorkerPoolBuilder[*InspectRPCRequest](lg, store, c.processInspectRPCReq).Build()

	c.workerPool = pool

	builder := component.NewComponentManagerBuilder()
	builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		distributor.Start(ctx)
		select {
		case <-ctx.Done():
		case <-distributor.Ready():
			ready()
		}
		<-distributor.Done()
	})
	for i := 0; i < c.config.NumberOfWorkers; i++ {
		builder.AddWorker(pool.WorkerLogic())
	}
	c.Component = builder.Build()
	return c, nil
}

// Inspect is called by gossipsub upon reception of a rpc from a remote  node.
// It creates a new InspectRPCRequest for the RPC to be inspected async by the worker pool.
func (c *ControlMsgValidationInspector) Inspect(from peer.ID, rpc *pubsub.RPC) error {
	// first truncate rpc
	err := c.truncateRPC(from, rpc)
	if err != nil {
		// irrecoverable error encountered
		c.logAndThrowError(fmt.Errorf("failed to get inspect RPC request could not perform truncation: %w", err))
	}
	// queue further async inspection
	req, err := NewInspectRPCRequest(from, rpc)
	if err != nil {
		c.logger.Error().
			Err(err).
			Bool(logging.KeyNetworkingSecurity, true).
			Str("peer_id", p2plogging.PeerId(from)).
			Msg("failed to get inspect RPC request")
		return fmt.Errorf("failed to get inspect RPC request: %w", err)
	}
	c.workerPool.Submit(req)

	return nil
}

// updateMetrics updates the metrics for the received RPC.
// Args:
//   - from: the sender.
//
// - rpc: the control message RPC.
func (c *ControlMsgValidationInspector) updateMetrics(from peer.ID, rpc *pubsub.RPC) {
	includedMessages := len(rpc.GetPublish())
	iHaveCount, iWantCount, graftCount, pruneCount := 0, 0, 0, 0
	ctl := rpc.GetControl()
	if ctl != nil {
		iHaveCount = len(ctl.GetIhave())
		iWantCount = len(ctl.GetIwant())
		graftCount = len(ctl.GetGraft())
		pruneCount = len(ctl.GetPrune())
	}
	c.metrics.OnIncomingRpcReceived(iHaveCount, iWantCount, graftCount, pruneCount, includedMessages)
	if c.logger.GetLevel() > zerolog.TraceLevel {
		return // skip logging if trace level is not enabled
	}
	c.logger.Trace().
		Str("peer_id", p2plogging.PeerId(from)).
		Int("iHaveCount", iHaveCount).
		Int("iWantCount", iWantCount).
		Int("graftCount", graftCount).
		Int("pruneCount", pruneCount).
		Int("included_message_count", includedMessages).
		Msg("received rpc with control messages")
}

// processInspectRPCReq func used by component workers to perform further inspection of RPC control messages that will validate ensure all control message
// types are valid in the RPC.
func (c *ControlMsgValidationInspector) processInspectRPCReq(req *InspectRPCRequest) error {
	c.updateMetrics(req.Peer, req.rpc)
	c.metrics.AsyncProcessingStarted()
	start := time.Now()
	defer func() {
		c.metrics.AsyncProcessingFinished(time.Since(start))
	}()

	activeClusterIDS := c.tracker.GetActiveClusterIds()
	for _, ctrlMsgType := range p2pmsg.ControlMessageTypes() {
		// iWant validation uses new sample size validation. This will be updated for all other control message types.
		switch ctrlMsgType {
		case p2pmsg.CtrlMsgGraft:
			err := c.inspectGraftMessages(req.Peer, req.rpc.GetControl().GetGraft(), activeClusterIDS)
			if err != nil {
				c.logAndDistributeAsyncInspectErrs(req, p2pmsg.CtrlMsgGraft, err)
				return nil
			}
		case p2pmsg.CtrlMsgPrune:
			err := c.inspectPruneMessages(req.Peer, req.rpc.GetControl().GetPrune(), activeClusterIDS)
			if err != nil {
				c.logAndDistributeAsyncInspectErrs(req, p2pmsg.CtrlMsgPrune, err)
				return nil
			}
		case p2pmsg.CtrlMsgIWant:
			err := c.inspectIWantMessages(req.Peer, req.rpc.GetControl().GetIwant())
			if err != nil {
				c.logAndDistributeAsyncInspectErrs(req, p2pmsg.CtrlMsgIWant, err)
				return nil
			}
		case p2pmsg.CtrlMsgIHave:
			err := c.inspectIHaveMessages(req.Peer, req.rpc.GetControl().GetIhave(), activeClusterIDS)
			if err != nil {
				c.logAndDistributeAsyncInspectErrs(req, p2pmsg.CtrlMsgIHave, err)
				return nil
			}
		}
	}

	return nil
}

// inspectGraftMessages performs topic validation on all grafts in the control message using the provided validateTopic func while tracking duplicates.
// Args:
// - from: peer ID of the sender.
// - grafts: the list of grafts to inspect.
// - activeClusterIDS: the list of active cluster ids.
// Returns:
// - DuplicateTopicErr: if there are any duplicate topics in the list of grafts
// - error: if any error occurs while sampling or validating topics, all returned errors are benign and should not cause the node to crash.
func (c *ControlMsgValidationInspector) inspectGraftMessages(from peer.ID, grafts []*pubsub_pb.ControlGraft, activeClusterIDS flow.ChainIDList) error {
	tracker := make(duplicateStrTracker)
	for _, graft := range grafts {
		topic := channels.Topic(graft.GetTopicID())
		if tracker.isDuplicate(topic.String()) {
			return NewDuplicateTopicErr(topic.String(), p2pmsg.CtrlMsgGraft)
		}
		tracker.set(topic.String())
		err := c.validateTopic(from, topic, activeClusterIDS)
		if err != nil {
			return err
		}
	}
	return nil
}

// inspectPruneMessages performs topic validation on all prunes in the control message using the provided validateTopic func while tracking duplicates.
// Args:
// - from: peer ID of the sender.
// - prunes: the list of iHaves to inspect.
// - activeClusterIDS: the list of active cluster ids.
// Returns:
//   - DuplicateTopicErr: if there are any duplicate topics found in the list of iHaves
//     or any duplicate message ids found inside a single iHave.
//   - error: if any error occurs while sampling or validating topics, all returned errors are benign and should not cause the node to crash.
func (c *ControlMsgValidationInspector) inspectPruneMessages(from peer.ID, prunes []*pubsub_pb.ControlPrune, activeClusterIDS flow.ChainIDList) error {
	tracker := make(duplicateStrTracker)
	for _, prune := range prunes {
		topic := channels.Topic(prune.GetTopicID())
		if tracker.isDuplicate(topic.String()) {
			return NewDuplicateTopicErr(topic.String(), p2pmsg.CtrlMsgPrune)
		}
		tracker.set(topic.String())
		err := c.validateTopic(from, topic, activeClusterIDS)
		if err != nil {
			return err
		}
	}
	return nil
}

// inspectIHaveMessages performs topic validation on all ihaves in the control message using the provided validateTopic func while tracking duplicates.
// Args:
// - from: peer ID of the sender.
// - iHaves: the list of iHaves to inspect.
// - activeClusterIDS: the list of active cluster ids.
// Returns:
//   - DuplicateTopicErr: if there are any duplicate topics found in the list of iHaves
//     or any duplicate message ids found inside a single iHave.
//   - error: if any error occurs while sampling or validating topics, all returned errors are benign and should not cause the node to crash.
func (c *ControlMsgValidationInspector) inspectIHaveMessages(from peer.ID, ihaves []*pubsub_pb.ControlIHave, activeClusterIDS flow.ChainIDList) error {
	if len(ihaves) == 0 {
		return nil
	}
	lg := c.logger.With().
		Str("peer_id", p2plogging.PeerId(from)).
		Int("sample_size", len(ihaves)).
		Int("max_sample_size", c.config.IHaveRPCInspectionConfig.MaxSampleSize).
		Logger()
	duplicateTopicTracker := make(duplicateStrTracker)
	duplicateMessageIDTracker := make(duplicateStrTracker)
	totalMessageIds := 0
	for _, ihave := range ihaves {
		messageIds := ihave.GetMessageIDs()
		topic := ihave.GetTopicID()
		if duplicateTopicTracker.isDuplicate(topic) {
			return NewDuplicateTopicErr(topic, p2pmsg.CtrlMsgIHave)
		}
		duplicateTopicTracker.set(topic)
		err := c.validateTopic(from, channels.Topic(topic), activeClusterIDS)
		if err != nil {
			return err
		}

		for _, messageID := range messageIds {
			if duplicateMessageIDTracker.isDuplicate(messageID) {
				return NewDuplicateTopicErr(messageID, p2pmsg.CtrlMsgIHave)
			}
			duplicateMessageIDTracker.set(messageID)
		}
	}
	lg.Debug().
		Int("total_message_ids", totalMessageIds).
		Msg("ihave control message validation complete")
	return nil
}

// inspectIWantMessages inspects RPC iWant control messages. This func will sample the iWants and perform validation on each iWant in the sample.
// Ensuring that the following are true:
// - Each iWant corresponds to an iHave that was sent.
// - Each topic in the iWant sample is a valid topic.
// If the number of iWants that do not have a corresponding iHave exceed the configured threshold an error is returned.
// Args:
// - from: peer ID of the sender.
// - iWant: the list of iWant control messages.
// Returns:
// - DuplicateTopicErr: if there are any duplicate message ids found in any of the iWants.
// - IWantCacheMissThresholdErr: if the rate of cache misses exceeds the configured allowed threshold.
// - error: if any error occurs while sampling or validating topics, all returned errors are benign and should not cause the node to crash.
func (c *ControlMsgValidationInspector) inspectIWantMessages(from peer.ID, iWants []*pubsub_pb.ControlIWant) error {
	if len(iWants) == 0 {
		return nil
	}
	lastHighest := c.rpcTracker.LastHighestIHaveRPCSize()
	lg := c.logger.With().
		Str("peer_id", p2plogging.PeerId(from)).
		Uint("max_sample_size", c.config.IWantRPCInspectionConfig.MaxSampleSize).
		Int64("last_highest_ihave_rpc_size", lastHighest).
		Logger()
	sampleSize := uint(len(iWants))
	tracker := make(duplicateStrTracker)
	cacheMisses := 0
	allowedCacheMissesThreshold := float64(sampleSize) * c.config.IWantRPCInspectionConfig.CacheMissThreshold
	duplicates := 0
	allowedDuplicatesThreshold := float64(sampleSize) * c.config.IWantRPCInspectionConfig.DuplicateMsgIDThreshold
	checkCacheMisses := len(iWants) > c.config.IWantRPCInspectionConfig.CacheMissCheckSize
	lg = lg.With().
		Uint("iwant_sample_size", sampleSize).
		Float64("allowed_cache_misses_threshold", allowedCacheMissesThreshold).
		Float64("allowed_duplicates_threshold", allowedDuplicatesThreshold).Logger()

	lg.Trace().Msg("validating sample of message ids from iwant control message")

	totalMessageIds := 0
	for _, iWant := range iWants {
		messageIds := iWant.GetMessageIDs()
		messageIDCount := uint(len(messageIds))
		for _, messageID := range messageIds {
			// check duplicate allowed threshold
			if tracker.isDuplicate(messageID) {
				duplicates++
				if float64(duplicates) > allowedDuplicatesThreshold {
					return NewIWantDuplicateMsgIDThresholdErr(duplicates, messageIDCount, c.config.IWantRPCInspectionConfig.DuplicateMsgIDThreshold)
				}
			}
			// check cache miss threshold
			if !c.rpcTracker.WasIHaveRPCSent(messageID) {
				cacheMisses++
				if checkCacheMisses {
					if float64(cacheMisses) > allowedCacheMissesThreshold {
						return NewIWantCacheMissThresholdErr(cacheMisses, messageIDCount, c.config.IWantRPCInspectionConfig.CacheMissThreshold)
					}
				}
			}
			tracker.set(messageID)
			totalMessageIds++
		}
	}

	lg.Debug().
		Int("total_message_ids", totalMessageIds).
		Int("cache_misses", cacheMisses).
		Int("duplicates", duplicates).
		Msg("iwant control message validation complete")

	return nil
}

// truncateRPC truncates the RPC by truncating each control message type using the configured max sample size values.
func (c *ControlMsgValidationInspector) truncateRPC(from peer.ID, rpc *pubsub.RPC) error {
	for _, ctlMsgType := range p2pmsg.ControlMessageTypes() {
		switch ctlMsgType {
		case p2pmsg.CtrlMsgGraft:
			c.truncateGraftMessages(rpc)
		case p2pmsg.CtrlMsgPrune:
			c.truncatePruneMessages(rpc)
		case p2pmsg.CtrlMsgIHave:
			c.truncateIHaveMessages(rpc)
		case p2pmsg.CtrlMsgIWant:
			c.truncateIWantMessages(from, rpc)
		default:
			// sanity check this should never happen
			c.logAndThrowError(fmt.Errorf("unknown control message type encountered during RPC truncation"))
		}
	}
	return nil
}

// truncateGraftMessages truncates the Graft control messages in the RPC. If the total number of Grafts in the RPC exceeds the configured
// GraftPruneMessageMaxSampleSize the list of Grafts will be truncated.
// Args:
//   - rpc: the rpc message to truncate.
//
// Returns:
//   - error: if any error encountered while sampling the messages, all errors are considered irrecoverable.
func (c *ControlMsgValidationInspector) truncateGraftMessages(rpc *pubsub.RPC) {
	grafts := rpc.GetControl().GetGraft()
	originalGraftSize := len(grafts)
	if originalGraftSize <= c.config.GraftPruneMessageMaxSampleSize {
		return // nothing to truncate
	}

	// truncate grafts and update metrics
	sampleSize := c.config.GraftPruneMessageMaxSampleSize
	c.performSample(p2pmsg.CtrlMsgGraft, uint(originalGraftSize), uint(sampleSize), func(i, j uint) {
		grafts[i], grafts[j] = grafts[j], grafts[i]
	})
	rpc.Control.Graft = grafts[:sampleSize]
	c.metrics.OnControlMessagesTruncated(p2pmsg.CtrlMsgGraft, originalGraftSize-len(rpc.Control.Graft))
}

// truncatePruneMessages truncates the Prune control messages in the RPC. If the total number of Prunes in the RPC exceeds the configured
// GraftPruneMessageMaxSampleSize the list of Prunes will be truncated.
// Args:
//   - rpc: the rpc message to truncate.
//
// Returns:
//   - error: if any error encountered while sampling the messages, all errors are considered irrecoverable.
func (c *ControlMsgValidationInspector) truncatePruneMessages(rpc *pubsub.RPC) {
	prunes := rpc.GetControl().GetPrune()
	originalPruneSize := len(prunes)
	if originalPruneSize <= c.config.GraftPruneMessageMaxSampleSize {
		return // nothing to truncate
	}

	sampleSize := c.config.GraftPruneMessageMaxSampleSize
	c.performSample(p2pmsg.CtrlMsgPrune, uint(originalPruneSize), uint(sampleSize), func(i, j uint) {
		prunes[i], prunes[j] = prunes[j], prunes[i]
	})
	rpc.Control.Prune = prunes[:sampleSize]
	c.metrics.OnControlMessagesTruncated(p2pmsg.CtrlMsgPrune, originalPruneSize-len(rpc.Control.Prune))
}

// truncateIHaveMessages truncates the iHaves control messages in the RPC. If the total number of iHaves in the RPC exceeds the configured
// MaxSampleSize the list of iHaves will be truncated.
// Args:
//   - rpc: the rpc message to truncate.
//
// Returns:
//   - error: if any error encountered while sampling the messages, all errors are considered irrecoverable.
func (c *ControlMsgValidationInspector) truncateIHaveMessages(rpc *pubsub.RPC) {
	ihaves := rpc.GetControl().GetIhave()
	originalIHaveCount := len(ihaves)
	if originalIHaveCount == 0 {
		return
	}

	if originalIHaveCount > c.config.IHaveRPCInspectionConfig.MaxSampleSize {
		// truncate ihaves and update metrics
		sampleSize := c.config.IHaveRPCInspectionConfig.MaxSampleSize
		if sampleSize > originalIHaveCount {
			sampleSize = originalIHaveCount
		}
		c.performSample(p2pmsg.CtrlMsgIHave, uint(originalIHaveCount), uint(sampleSize), func(i, j uint) {
			ihaves[i], ihaves[j] = ihaves[j], ihaves[i]
		})
		rpc.Control.Ihave = ihaves[:sampleSize]
		c.metrics.OnControlMessagesTruncated(p2pmsg.CtrlMsgIHave, originalIHaveCount-len(rpc.Control.Ihave))
	}
	c.truncateIHaveMessageIds(rpc)
}

// truncateIHaveMessageIds truncates the message ids for each iHave control message in the RPC. If the total number of message ids in a single iHave exceeds the configured
// MaxMessageIDSampleSize the list of message ids will be truncated. Before message ids are truncated the iHave control messages should have been truncated themselves.
// Args:
//   - rpc: the rpc message to truncate.
func (c *ControlMsgValidationInspector) truncateIHaveMessageIds(rpc *pubsub.RPC) {
	for _, ihave := range rpc.GetControl().GetIhave() {
		messageIDs := ihave.GetMessageIDs()
		originalMessageIdCount := len(messageIDs)
		if originalMessageIdCount == 0 {
			continue // nothing to truncate; skip
		}

		if originalMessageIdCount > c.config.IHaveRPCInspectionConfig.MaxMessageIDSampleSize {
			sampleSize := c.config.IHaveRPCInspectionConfig.MaxMessageIDSampleSize
			if sampleSize > originalMessageIdCount {
				sampleSize = originalMessageIdCount
			}
			c.performSample(p2pmsg.CtrlMsgIHave, uint(originalMessageIdCount), uint(sampleSize), func(i, j uint) {
				messageIDs[i], messageIDs[j] = messageIDs[j], messageIDs[i]
			})
			ihave.MessageIDs = messageIDs[:sampleSize]
			c.metrics.OnIHaveControlMessageIdsTruncated(originalMessageIdCount - len(ihave.MessageIDs))
		}
		c.metrics.OnIHaveMessageIDsReceived(ihave.GetTopicID(), len(ihave.MessageIDs))
	}
}

// truncateIWantMessages truncates the iWant control messages in the RPC. If the total number of iWants in the RPC exceeds the configured
// MaxSampleSize the list of iWants will be truncated.
// Args:
//   - rpc: the rpc message to truncate.
//
// Returns:
//   - error: if any error encountered while sampling the messages, all errors are considered irrecoverable.
func (c *ControlMsgValidationInspector) truncateIWantMessages(from peer.ID, rpc *pubsub.RPC) {
	iWants := rpc.GetControl().GetIwant()
	originalIWantCount := uint(len(iWants))
	if originalIWantCount == 0 {
		return
	}

	if originalIWantCount > c.config.IWantRPCInspectionConfig.MaxSampleSize {
		// truncate iWants and update metrics
		sampleSize := c.config.IWantRPCInspectionConfig.MaxSampleSize
		if sampleSize > originalIWantCount {
			sampleSize = originalIWantCount
		}
		c.performSample(p2pmsg.CtrlMsgIWant, originalIWantCount, sampleSize, func(i, j uint) {
			iWants[i], iWants[j] = iWants[j], iWants[i]
		})
		rpc.Control.Iwant = iWants[:sampleSize]
		c.metrics.OnControlMessagesTruncated(p2pmsg.CtrlMsgIWant, int(originalIWantCount)-len(rpc.Control.Iwant))
	}
	c.truncateIWantMessageIds(from, rpc)
}

// truncateIWantMessageIds truncates the message ids for each iWant control message in the RPC. If the total number of message ids in a single iWant exceeds the configured
// MaxMessageIDSampleSize the list of message ids will be truncated. Before message ids are truncated the iWant control messages should have been truncated themselves.
// Args:
//   - rpc: the rpc message to truncate.
//
// Returns:
//   - error: if any error encountered while sampling the messages, all errors are considered irrecoverable.
func (c *ControlMsgValidationInspector) truncateIWantMessageIds(from peer.ID, rpc *pubsub.RPC) {
	lastHighest := c.rpcTracker.LastHighestIHaveRPCSize()
	lg := c.logger.With().
		Str("peer_id", p2plogging.PeerId(from)).
		Uint("max_sample_size", c.config.IWantRPCInspectionConfig.MaxSampleSize).
		Int64("last_highest_ihave_rpc_size", lastHighest).
		Logger()

	sampleSize := int(10 * lastHighest)
	if sampleSize == 0 || sampleSize > c.config.IWantRPCInspectionConfig.MaxMessageIDSampleSize {
		// invalid or 0 sample size is suspicious
		lg.Warn().Str(logging.KeySuspicious, "true").Msg("zero or invalid sample size, using default max sample size")
		sampleSize = c.config.IWantRPCInspectionConfig.MaxMessageIDSampleSize
	}
	for _, iWant := range rpc.GetControl().GetIwant() {
		messageIDs := iWant.GetMessageIDs()
		totalMessageIdCount := len(messageIDs)
		if totalMessageIdCount == 0 {
			continue // nothing to truncate; skip
		}

		if totalMessageIdCount > sampleSize {
			c.performSample(p2pmsg.CtrlMsgIWant, uint(totalMessageIdCount), uint(sampleSize), func(i, j uint) {
				messageIDs[i], messageIDs[j] = messageIDs[j], messageIDs[i]
			})
			iWant.MessageIDs = messageIDs[:sampleSize]
			c.metrics.OnIWantControlMessageIdsTruncated(totalMessageIdCount - len(iWant.MessageIDs))
		}
		c.metrics.OnIWantMessageIDsReceived(len(iWant.MessageIDs))
	}
}

// Name returns the name of the rpc inspector.
func (c *ControlMsgValidationInspector) Name() string {
	return rpcInspectorComponentName
}

// ActiveClustersChanged consumes cluster ID update protocol events.
func (c *ControlMsgValidationInspector) ActiveClustersChanged(clusterIDList flow.ChainIDList) {
	c.tracker.StoreActiveClusterIds(clusterIDList)
}

// performSample performs sampling on the specified control message that will randomize
// the items in the control message slice up to index sampleSize-1. Any error encountered during sampling is considered
// irrecoverable and will cause the node to crash.
func (c *ControlMsgValidationInspector) performSample(ctrlMsg p2pmsg.ControlMessageType, totalSize, sampleSize uint, swap func(i, j uint)) {
	err := flowrand.Samples(totalSize, sampleSize, swap)
	if err != nil {
		c.logAndThrowError(fmt.Errorf("failed to get random sample of %s control messages: %w", ctrlMsg, err))
	}
}

// validateTopic ensures the topic is a valid flow topic/channel.
// Expected error returns during normal operations:
//   - channels.InvalidTopicErr: if topic is invalid.
//   - ErrActiveClusterIdsNotSet: if the cluster ID provider is not set.
//   - channels.UnknownClusterIDErr: if the topic contains a cluster ID prefix that is not in the active cluster IDs list.
//
// This func returns an exception in case of unexpected bug or state corruption if cluster prefixed topic validation
// fails due to unexpected error returned when getting the active cluster IDS.
func (c *ControlMsgValidationInspector) validateTopic(from peer.ID, topic channels.Topic, activeClusterIds flow.ChainIDList) error {
	channel, ok := channels.ChannelFromTopic(topic)
	if !ok {
		return channels.NewInvalidTopicErr(topic, fmt.Errorf("failed to get channel from topic"))
	}

	// handle cluster prefixed topics
	if channels.IsClusterChannel(channel) {
		return c.validateClusterPrefixedTopic(from, topic, activeClusterIds)
	}

	// non cluster prefixed topic validation
	err := channels.IsValidNonClusterFlowTopic(topic, c.sporkID)
	if err != nil {
		return err
	}
	return nil
}

// validateClusterPrefixedTopic validates cluster prefixed topics.
// Expected error returns during normal operations:
//   - ErrActiveClusterIdsNotSet: if the cluster ID provider is not set.
//   - channels.InvalidTopicErr: if topic is invalid.
//   - channels.UnknownClusterIDErr: if the topic contains a cluster ID prefix that is not in the active cluster IDs list.
//
// In the case where an ErrActiveClusterIdsNotSet or UnknownClusterIDErr is encountered and the cluster prefixed topic received
// tracker for the peer is less than or equal to the configured ClusterPrefixHardThreshold an error will only be logged and not returned.
// At the point where the hard threshold is crossed the error will be returned and the sender will start to be penalized.
// Any errors encountered while incrementing or loading the cluster prefixed control message gauge for a peer will result in an irrecoverable error being thrown, these
// errors are unexpected and irrecoverable indicating a bug.
func (c *ControlMsgValidationInspector) validateClusterPrefixedTopic(from peer.ID, topic channels.Topic, activeClusterIds flow.ChainIDList) error {
	lg := c.logger.With().
		Str("from", p2plogging.PeerId(from)).
		Logger()
	// reject messages from unstaked nodes for cluster prefixed topics
	nodeID, err := c.getFlowIdentifier(from)
	if err != nil {
		return err
	}

	if len(activeClusterIds) == 0 {
		// cluster IDs have not been updated yet
		_, incErr := c.tracker.Inc(nodeID)
		if incErr != nil {
			// irrecoverable error encountered
			c.logAndThrowError(fmt.Errorf("error encountered while incrementing the cluster prefixed control message gauge %s: %w", nodeID, err))
		}

		// if the amount of messages received is below our hard threshold log the error and return nil.
		if c.checkClusterPrefixHardThreshold(nodeID) {
			lg.Warn().
				Err(err).
				Str("topic", topic.String()).
				Msg("failed to validate cluster prefixed control message with cluster pre-fixed topic active cluster ids not set")
			return nil
		}

		return NewActiveClusterIdsNotSetErr(topic)
	}

	err = channels.IsValidFlowClusterTopic(topic, activeClusterIds)
	if err != nil {
		if channels.IsUnknownClusterIDErr(err) {
			// unknown cluster ID error could indicate that a node has fallen
			// behind and needs to catchup increment to topics received cache.
			_, incErr := c.tracker.Inc(nodeID)
			if incErr != nil {
				c.logAndThrowError(fmt.Errorf("error encountered while incrementing the cluster prefixed control message gauge %s: %w", nodeID, err))
			}
			// if the amount of messages received is below our hard threshold log the error and return nil.
			if c.checkClusterPrefixHardThreshold(nodeID) {
				lg.Warn().
					Err(err).
					Str("topic", topic.String()).
					Msg("processing unknown cluster prefixed topic received below cluster prefixed discard threshold peer may be behind in the protocol")
				return nil
			}
		}
		return err
	}

	return nil
}

// getFlowIdentifier returns the flow identity identifier for a peer.
// Args:
//   - peerID: the peer id of the sender.
//
// The returned error indicates that the peer is un-staked.
func (c *ControlMsgValidationInspector) getFlowIdentifier(peerID peer.ID) (flow.Identifier, error) {
	id, ok := c.idProvider.ByPeerID(peerID)
	if !ok {
		return flow.ZeroID, NewUnstakedPeerErr(fmt.Errorf("failed to get flow identity for peer: %s", peerID))
	}
	return id.ID(), nil
}

// checkClusterPrefixHardThreshold returns true if the cluster prefix received tracker count is less than
// the configured ClusterPrefixHardThreshold, false otherwise.
// If any error is encountered while loading from the tracker this func will throw an error on the signaler context, these errors
// are unexpected and irrecoverable indicating a bug.
func (c *ControlMsgValidationInspector) checkClusterPrefixHardThreshold(nodeID flow.Identifier) bool {
	gauge, err := c.tracker.Load(nodeID)
	if err != nil {
		// irrecoverable error encountered
		c.logAndThrowError(fmt.Errorf("cluster prefixed control message gauge during hard threshold check failed for node %s: %w", nodeID, err))
	}
	return gauge <= c.config.ClusterPrefixHardThreshold
}

// logAndDistributeErr logs the provided error and attempts to disseminate an invalid control message validation notification for the error.
func (c *ControlMsgValidationInspector) logAndDistributeAsyncInspectErrs(req *InspectRPCRequest, ctlMsgType p2pmsg.ControlMessageType, err error) {
	lg := c.logger.With().
		Err(err).
		Str("control_message_type", ctlMsgType.String()).
		Bool(logging.KeySuspicious, true).
		Bool(logging.KeyNetworkingSecurity, true).
		Str("peer_id", p2plogging.PeerId(req.Peer)).
		Logger()

	switch {
	case IsErrActiveClusterIDsNotSet(err):
		lg.Warn().Msg("active cluster ids not set")
	case IsErrUnstakedPeer(err):
		lg.Warn().Msg("control message received from unstaked peer")
	default:
		distErr := c.distributor.Distribute(p2p.NewInvalidControlMessageNotification(req.Peer, ctlMsgType, err))
		if distErr != nil {
			lg.Error().
				Err(distErr).
				Msg("failed to distribute invalid control message notification")
		}
		lg.Error().Msg("rpc control message async inspection failed")
	}
}

func (c *ControlMsgValidationInspector) logAndThrowError(err error) {
	c.logger.Error().
		Err(err).
		Bool(logging.KeySuspicious, true).
		Bool(logging.KeyNetworkingSecurity, true).
		Msg("unexpected irrecoverable error encountered")
	c.ctx.Throw(err)
}
