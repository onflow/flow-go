package validation

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/engine/common/worker"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/internal/cache"
	"github.com/onflow/flow-go/network/p2p/inspector/internal/ratelimit"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
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
	rpcTracker   p2p.RPCControlTracking
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
func NewControlMsgValidationInspector(
	logger zerolog.Logger,
	sporkID flow.Identifier,
	config *p2pconf.GossipSubRPCValidationInspectorConfigs,
	distributor p2p.GossipSubInspectorNotifDistributor,
	inspectMsgQueueCacheCollector module.HeroCacheMetrics,
	clusterPrefixedCacheCollector module.HeroCacheMetrics,
	idProvider module.IdentityProvider,
	inspectorMetrics module.GossipSubRpcValidationInspectorMetrics,
	rpcTracker p2p.RPCControlTracking) (*ControlMsgValidationInspector, error) {
	lg := logger.With().Str("component", "gossip_sub_rpc_validation_inspector").Logger()

	clusterPrefixedTracker, err := cache.NewClusterPrefixedMessagesReceivedTracker(logger, config.ClusterPrefixedControlMsgsReceivedCacheSize, clusterPrefixedCacheCollector, config.ClusterPrefixedControlMsgsReceivedCacheDecay)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster prefix topics received tracker")
	}

	c := &ControlMsgValidationInspector{
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
	// start rate limiters cleanup loop in workers
	for _, conf := range c.config.AllCtrlMsgValidationConfig() {
		l := logger.With().Str("control_message_type", conf.ControlMsg.String()).Logger()
		limiter := ratelimit.NewControlMessageRateLimiter(l, rate.Limit(conf.RateLimit), conf.RateLimit)
		c.rateLimiters[conf.ControlMsg] = limiter
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			limiter.Start(ctx)
		})
	}
	for i := 0; i < c.config.NumberOfWorkers; i++ {
		builder.AddWorker(pool.WorkerLogic())
	}
	c.Component = builder.Build()
	return c, nil
}

// Inspect is called by gossipsub upon reception of an rpc from a remote  node.
// It creates a new InspectRPCRequest for the RPC to be inspected async by the worker pool.
func (c *ControlMsgValidationInspector) Inspect(from peer.ID, rpc *pubsub.RPC) error {
	// queue further async inspection
	req, err := NewInspectRPCRequest(from, rpc)
	if err != nil {
		c.logger.Error().
			Err(err).
			Str("peer_id", from.String()).
			Msg("failed to get inspect RPC request")
		return fmt.Errorf("failed to get inspect RPC request: %w", err)
	}
	c.workerPool.Submit(req)
	return nil
}

// inspectIWant inspects RPC iWant control messages. This func will sample the iWants and perform validation on each iWant in the sample.
// Ensuring that the following are true:
// - Each iWant corresponds to an iHave that was sent.
// - Each topic in the iWant sample is a valid topic.
// If the number of iWants that do not have a corresponding iHave exceed the configured threshold an error is returned.
// Args:
// - iWant: the list of iWant control messages.
// Returns:
// - DuplicateFoundErr: if there are any duplicate message ids found in across any of the iWants.
// - IWantCacheMissThresholdErr: if the rate of cache misses exceeds the configured allowed threshold.
// - error: if any error occurs while sampling the iWants
func (c *ControlMsgValidationInspector) inspectIWant(iWants []*pubsub_pb.ControlIWant) error {
	if len(iWants) == 0 {
		return nil
	}
	sampleSize := uint(10 * c.rpcTracker.LastHighestIHaveRPCSize())
	if sampleSize == 0 || sampleSize > c.config.IWantRPCInspectionConfig.MaxSampleSize {
		c.logger.Warn().
			Uint("sample_size", sampleSize).
			Uint("max_sample_size", c.config.IWantRPCInspectionConfig.MaxSampleSize).
			Str(logging.KeySuspicious, "true"). // max sample size is suspicious
			Str(logging.KeyNetworkingSecurity, "true"). // zero sample size is a security hole
			Msg("zero or invalid sample size, using default max sample size")
		sampleSize = c.config.IWantRPCInspectionConfig.MaxSampleSize
	}

	var iWantMsgIDPool []string
	// opens all iWant boxes into a sample pool to be sampled.
	for _, iWant := range iWants {
		if len(iWant.GetMessageIDs()) == 0 {
			continue
		}
		iWantMsgIDPool = append(iWantMsgIDPool, iWant.GetMessageIDs()...)
	}

	if sampleSize > uint(len(iWantMsgIDPool)) {
		sampleSize = uint(len(iWantMsgIDPool))
	}

	swap := func(i, j uint) {
		iWantMsgIDPool[i], iWantMsgIDPool[j] = iWantMsgIDPool[j], iWantMsgIDPool[i]
	}

	err := c.performSample(p2pmsg.CtrlMsgIWant, uint(len(iWantMsgIDPool)), sampleSize, swap)
	if err != nil {
		c.logger.Fatal().Err(fmt.Errorf("failed to sample iwant messages: %w", err)).Msg("irrecoverable error encountered while sampling iwant control messages")
	}

	tracker := make(duplicateStrTracker)
	cacheMisses := float64(0)
	for _, messageID := range iWantMsgIDPool[:sampleSize] {
		if tracker.isDuplicate(messageID) {
			return NewDuplicateFoundErr(fmt.Errorf("duplicate message ID found: %s", messageID))
		}
		if !c.rpcTracker.WasIHaveRPCSent(messageID) {
			cacheMisses++
		}
		tracker.set(messageID)
	}

	// check cache miss rate
	if cacheMisses/float64(sampleSize) > c.config.IWantRPCInspectionConfig.CacheMissThreshold {
		return NewIWantCacheMissThresholdErr(cacheMisses, float64(sampleSize), c.config.IWantRPCInspectionConfig.CacheMissThreshold)
	}

	return nil
}

// inspectGraft performs topic validation on all grafts in the control message using the provided validateTopic func while tracking duplicates.
func (c *ControlMsgValidationInspector) inspectGraft(from peer.ID, grafts []*pubsub_pb.ControlGraft, activeClusterIDS flow.ChainIDList) error {
	totalGrafts := len(grafts)
	if totalGrafts == 0 {
		return nil
	}
	sampleSize := c.config.ControlMessageMaxSampleSize
	if sampleSize > totalGrafts {
		sampleSize = totalGrafts
	}

	err := c.performSample(p2pmsg.CtrlMsgGraft, uint(totalGrafts), uint(sampleSize), func(i, j uint) {
		grafts[i], grafts[j] = grafts[j], grafts[i]
	})
	if err != nil {
		return fmt.Errorf("failed to sample ihave messages: %w", err)
	}

	tracker := make(duplicateStrTracker)
	for _, graft := range grafts[:sampleSize] {
		topic := channels.Topic(graft.GetTopicID())
		if tracker.isDuplicate(topic.String()) {
			return NewDuplicateFoundErr(fmt.Errorf("duplicate topic found: %s", topic.String()))
		}
		tracker.set(topic.String())
		err = c.validateTopic(from, topic, activeClusterIDS)
		if err != nil {
			return err
		}
	}
	return nil
}

// inspectPrune performs topic validation on all prunes in the control message using the provided validateTopic func while tracking duplicates.
func (c *ControlMsgValidationInspector) inspectPrune(from peer.ID, prunes []*pubsub_pb.ControlPrune, activeClusterIDS flow.ChainIDList) error {
	totalPrunes := len(prunes)
	if totalPrunes == 0 {
		return nil
	}
	sampleSize := c.config.ControlMessageMaxSampleSize
	if sampleSize > totalPrunes {
		sampleSize = totalPrunes
	}

	err := c.performSample(p2pmsg.CtrlMsgPrune, uint(totalPrunes), uint(sampleSize), func(i, j uint) {
		prunes[i], prunes[j] = prunes[j], prunes[i]
	})
	if err != nil {
		return fmt.Errorf("failed to sample ihave messages: %w", err)
	}

	tracker := make(duplicateStrTracker)
	for _, prune := range prunes[:sampleSize] {
		topic := channels.Topic(prune.GetTopicID())
		if tracker.isDuplicate(topic.String()) {
			return NewDuplicateFoundErr(fmt.Errorf("duplicate topic found: %s", topic.String()))
		}
		tracker.set(topic.String())
		err = c.validateTopic(from, topic, activeClusterIDS)
		if err != nil {
			return err
		}
	}
	return nil
}

// inspectIhave performs topic validation on all ihaves in the control message using the provided validateTopic func while tracking duplicates.
func (c *ControlMsgValidationInspector) inspectIhave(from peer.ID, ihaves []*pubsub_pb.ControlIHave, activeClusterIDS flow.ChainIDList) error {
	totalIHaves := len(ihaves)
	if totalIHaves == 0 {
		return nil
	}
	sampleSize := c.config.IHaveMaxSampleSize
	if sampleSize > totalIHaves {
		sampleSize = totalIHaves
	}

	err := c.performSample(p2pmsg.CtrlMsgIHave, uint(totalIHaves), uint(sampleSize), func(i, j uint) {
		ihaves[i], ihaves[j] = ihaves[j], ihaves[i]
	})
	if err != nil {
		return fmt.Errorf("failed to sample ihave messages: %w", err)
	}

	tracker := make(duplicateStrTracker)
	for _, ihave := range ihaves[:sampleSize] {
		topic := channels.Topic(ihave.GetTopicID())
		if tracker.isDuplicate(topic.String()) {
			return NewDuplicateFoundErr(fmt.Errorf("duplicate topic found: %s", topic.String()))
		}
		tracker.set(topic.String())
		err = c.validateTopic(from, topic, activeClusterIDS)
		if err != nil {
			return err
		}
	}
	return nil
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
// the items in the control message slice up to index sampleSize-1.
func (c *ControlMsgValidationInspector) performSample(ctrlMsg p2pmsg.ControlMessageType, totalSize, sampleSize uint, swap func(i, j uint)) error {
	err := flowrand.Samples(totalSize, sampleSize, swap)
	if err != nil {
		return fmt.Errorf("failed to get random sample of %s control messages: %w", ctrlMsg, err)
	}
	return nil
}

// processInspectRPCReq func used by component workers to perform further inspection of RPC control messages that will validate ensure all control message
// types are valid in the RPC.
func (c *ControlMsgValidationInspector) processInspectRPCReq(req *InspectRPCRequest) error {
	c.metrics.AsyncProcessingStarted()
	start := time.Now()
	defer func() {
		c.metrics.AsyncProcessingFinished(time.Since(start))
	}()

	activeClusterIDS := c.tracker.GetActiveClusterIds()
	var errs *multierror.Error
	for _, ctrlMsgType := range p2pmsg.ControlMessageTypes() {
		// iWant validation uses new sample size validation. This will be updated for all other control message types.
		switch ctrlMsgType {
		case p2pmsg.CtrlMsgGraft:
			err := c.inspectGraft(req.Peer, req.rpc.GetControl().GetGraft(), activeClusterIDS)
			if err != nil {
				errs = multierror.Append(errs, err)
			}
		case p2pmsg.CtrlMsgPrune:
			err := c.inspectPrune(req.Peer, req.rpc.GetControl().GetPrune(), activeClusterIDS)
			if err != nil {
				errs = multierror.Append(errs, err)
			}
		case p2pmsg.CtrlMsgIWant:
			err := c.inspectIhave(req.Peer, req.rpc.GetControl().GetIhave(), activeClusterIDS)
			if err != nil {
				errs = multierror.Append(errs, err)
			}
		case p2pmsg.CtrlMsgIHave:
			err := c.inspectIWant(req.rpc.GetControl().GetIwant())
			if err != nil {
				errs = multierror.Append(errs, err)
			}
		}
	}

	c.logAndDistributeAsyncInspectErrs(req, errs)
	return nil
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
// Any errors encountered while incrementing or loading the cluster prefixed control message gauge for a peer will result in a fatal log, these
// errors are unexpected and irrecoverable indicating a bug.
func (c *ControlMsgValidationInspector) validateClusterPrefixedTopic(from peer.ID, topic channels.Topic, activeClusterIds flow.ChainIDList) error {
	lg := c.logger.With().
		Str("from", from.String()).
		Logger()
	// reject messages from unstaked nodes for cluster prefixed topics
	nodeID, err := c.getFlowIdentifier(from)
	if err != nil {
		return err
	}

	if len(activeClusterIds) == 0 {
		// cluster IDs have not been updated yet
		_, err = c.tracker.Inc(nodeID)
		if err != nil {
			return err
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
				// irrecoverable error encountered
				c.logger.Fatal().Err(incErr).
					Str("node_id", nodeID.String()).
					Msg("unexpected irrecoverable error encountered while incrementing the cluster prefixed control message gauge")
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
// If any error is encountered while loading from the tracker this func will emit a fatal level log, these errors
// are unexpected and irrecoverable indicating a bug.
func (c *ControlMsgValidationInspector) checkClusterPrefixHardThreshold(nodeID flow.Identifier) bool {
	gauge, err := c.tracker.Load(nodeID)
	if err != nil {
		// irrecoverable error encountered
		c.logger.Fatal().Err(err).
			Str("node_id", nodeID.String()).
			Msg("unexpected irrecoverable error encountered while loading the cluster prefixed control message gauge during hard threshold check")
	}
	return gauge <= c.config.ClusterPrefixHardThreshold
}

// logAndDistributeErr logs the provided error and attempts to disseminate an invalid control message validation notification for the error.
func (c *ControlMsgValidationInspector) logAndDistributeAsyncInspectErrs(req *InspectRPCRequest, errs *multierror.Error) {
	lg := c.logger.With().
		Bool(logging.KeySuspicious, true).
		Str("peer_id", req.Peer.String()).
		Int("error_count", errs.Len()).
		Logger()

	lg.Error().Err(errs).Msg("rpc control message async inspection failed")

	err := c.distributor.Distribute(p2p.NewInvalidControlMessageNotification(req.Peer, "", uint64(errs.Len()), errs))
	if err != nil {
		lg.Error().
			Err(err).
			Msg("failed to distribute invalid control message notification")
	}
}
