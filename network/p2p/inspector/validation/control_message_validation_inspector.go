package validation

import (
	"fmt"
	"time"

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
	"github.com/onflow/flow-go/module/util"
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
	// workerPool queue that stores *InspectMsgRequest that will be processed by component workers.
	workerPool *worker.Pool[*InspectMsgRequest]
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
	inspectorMetrics module.GossipSubRpcValidationInspectorMetrics) (*ControlMsgValidationInspector, error) {
	lg := logger.With().Str("component", "gossip_sub_rpc_validation_inspector").Logger()

	tracker, err := cache.NewClusterPrefixedMessagesReceivedTracker(logger, config.ClusterPrefixedControlMsgsReceivedCacheSize, clusterPrefixedCacheCollector, config.ClusterPrefixedControlMsgsReceivedCacheDecay)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster prefix topics received tracker")
	}

	c := &ControlMsgValidationInspector{
		logger:       lg,
		sporkID:      sporkID,
		config:       config,
		distributor:  distributor,
		tracker:      tracker,
		idProvider:   idProvider,
		metrics:      inspectorMetrics,
		rateLimiters: make(map[p2pmsg.ControlMessageType]p2p.BasicRateLimiter),
	}

	store := queue.NewHeroStore(config.CacheSize, logger, inspectMsgQueueCacheCollector)
	pool := worker.NewWorkerPoolBuilder[*InspectMsgRequest](lg, store, c.processInspectMsgReq).Build()

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

// Inspect is called by gossipsub upon reception of an rpc from a remote node.
// It examines the provided message to ensure it adheres to the expected
// format and conventions. If the message passes validation, the method returns
// a nil error. If an issue is found, the method returns an error detailing
// the specific issue encountered.
// The returned error can be of two types:
//  1. Expected errors: These are issues that are expected to occur during normal
//     operation, such as invalid messages or messages that don't follow the
//     conventions. These errors should be handled gracefully by the caller.
//  2. Exceptions: These are unexpected issues, such as internal system errors
//     or misconfigurations, that may require immediate attention or a change in
//     the system's behavior. The caller should log and handle these errors
//     accordingly.
//
// The returned error is returned to the gossipsub node which causes the rejection of rpc (for non-nil errors).
func (c *ControlMsgValidationInspector) Inspect(from peer.ID, rpc *pubsub.RPC) error {
	control := rpc.GetControl()
	for _, ctrlMsgType := range p2pmsg.ControlMessageTypes() {
		lg := c.logger.With().
			Str("peer_id", from.String()).
			Str("ctrl_msg_type", string(ctrlMsgType)).Logger()
		validationConfig, ok := c.config.GetCtrlMsgValidationConfig(ctrlMsgType)
		if !ok {
			lg.Trace().Msg("validation configuration for control type does not exists skipping")
			continue
		}

		switch ctrlMsgType {
		case p2pmsg.CtrlMsgGraft, p2pmsg.CtrlMsgPrune:
			// normal pre-processing
			err := c.blockingPreprocessingRpc(from, validationConfig, control)
			if err != nil {
				lg.Error().
					Err(err).
					Msg("could not pre-process rpc, aborting")
				return fmt.Errorf("could not pre-process rpc, aborting: %w", err)
			}
		case p2pmsg.CtrlMsgIHave:
			// iHave specific pre-processing
			sampleSize := util.SampleN(len(control.GetIhave()), c.config.IHaveInspectionMaxSampleSize, c.config.IHaveSyncInspectSampleSizePercentage)
			err := c.blockingIHaveSamplePreprocessing(from, validationConfig, control, sampleSize)
			if err != nil {
				lg.Error().
					Err(err).
					Msg("could not pre-process rpc, aborting")
				return fmt.Errorf("could not pre-process rpc, aborting: %w", err)
			}
		}

		// queue further async inspection
		req, err := NewInspectMsgRequest(from, validationConfig, control)
		if err != nil {
			lg.Error().
				Err(err).
				Str("peer_id", from.String()).
				Str("ctrl_msg_type", string(ctrlMsgType)).
				Msg("failed to get inspect message request")
			return fmt.Errorf("failed to get inspect message request: %w", err)
		}
		c.workerPool.Submit(req)
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

// blockingPreprocessingRpc ensures the RPC control message count does not exceed the configured discard threshold.
// Expected error returns during normal operations:
//   - ErrDiscardThreshold: if control message count exceeds the configured discard threshold.
//
// blockingPreprocessingRpc generic pre-processing validation func that ensures the RPC control message count does not exceed the configured hard threshold.
func (c *ControlMsgValidationInspector) blockingPreprocessingRpc(from peer.ID, validationConfig *p2pconf.CtrlMsgValidationConfig, controlMessage *pubsub_pb.ControlMessage) error {
	if validationConfig.ControlMsg != p2pmsg.CtrlMsgGraft && validationConfig.ControlMsg != p2pmsg.CtrlMsgPrune {
		return fmt.Errorf("unexpected control message type %s encountered during blocking pre-processing rpc, expected %s or %s", validationConfig.ControlMsg, p2pmsg.CtrlMsgGraft, p2pmsg.CtrlMsgPrune)
	}
	count := c.getCtrlMsgCount(validationConfig.ControlMsg, controlMessage)
	lg := c.logger.With().
		Uint64("ctrl_msg_count", count).
		Str("peer_id", from.String()).
		Str("ctrl_msg_type", string(validationConfig.ControlMsg)).Logger()

	c.metrics.BlockingPreProcessingStarted(validationConfig.ControlMsg.String(), uint(count))
	start := time.Now()
	defer func() {
		c.metrics.BlockingPreProcessingFinished(validationConfig.ControlMsg.String(), uint(count), time.Since(start))
	}()

	// if Count greater than hard threshold drop message and penalize
	if count > validationConfig.HardThreshold {
		hardThresholdErr := NewHardThresholdErr(validationConfig.ControlMsg, count, validationConfig.HardThreshold)
		lg.Warn().
			Err(hardThresholdErr).
			Uint64("upper_threshold", hardThresholdErr.hardThreshold).
			Bool(logging.KeySuspicious, true).
			Msg("rejecting rpc control message")
		err := c.distributor.Distribute(p2p.NewInvalidControlMessageNotification(from, validationConfig.ControlMsg, count, hardThresholdErr))
		if err != nil {
			lg.Error().
				Err(err).
				Bool(logging.KeySuspicious, true).
				Msg("failed to distribute invalid control message notification")
			return err
		}
		return hardThresholdErr
	}

	return nil
}

// blockingPreprocessingSampleRpc blocking pre-processing of a sample of iHave control messages.
func (c *ControlMsgValidationInspector) blockingIHaveSamplePreprocessing(from peer.ID, validationConfig *p2pconf.CtrlMsgValidationConfig, controlMessage *pubsub_pb.ControlMessage, sampleSize uint) error {
	c.metrics.BlockingPreProcessingStarted(p2pmsg.CtrlMsgIHave.String(), sampleSize)
	start := time.Now()
	defer func() {
		c.metrics.BlockingPreProcessingFinished(p2pmsg.CtrlMsgIHave.String(), sampleSize, time.Since(start))
	}()
	err := c.blockingPreprocessingSampleRpc(from, validationConfig, controlMessage, sampleSize)
	if err != nil {
		return fmt.Errorf("failed to pre-process a sample of iHave messages: %w", err)
	}
	return nil
}

// blockingPreprocessingSampleRpc blocking pre-processing validation func that performs some pre-validation of RPC control messages.
// If the RPC control message count exceeds the configured hard threshold we perform synchronous topic validation on a subset
// of the control messages. This is used for control message types that do not have an upper bound on the amount of messages a node can send.
func (c *ControlMsgValidationInspector) blockingPreprocessingSampleRpc(from peer.ID, validationConfig *p2pconf.CtrlMsgValidationConfig, controlMessage *pubsub_pb.ControlMessage, sampleSize uint) error {
	if validationConfig.ControlMsg != p2pmsg.CtrlMsgIHave && validationConfig.ControlMsg != p2pmsg.CtrlMsgIWant {
		return fmt.Errorf("unexpected control message type %s encountered during blocking pre-processing sample rpc, expected %s or %s", validationConfig.ControlMsg, p2pmsg.CtrlMsgIHave, p2pmsg.CtrlMsgIWant)
	}
	activeClusterIDS := c.tracker.GetActiveClusterIds()
	count := c.getCtrlMsgCount(validationConfig.ControlMsg, controlMessage)
	lg := c.logger.With().
		Uint64("ctrl_msg_count", count).
		Str("peer_id", from.String()).
		Str("ctrl_msg_type", string(validationConfig.ControlMsg)).Logger()
	// if count greater than hard threshold perform synchronous topic validation on random subset of the iHave messages
	if count > validationConfig.HardThreshold {
		// for iHave control message topic validation we only validate a random subset of the messages
		// shuffle the ihave messages to perform random validation on a subset of size sampleSize
		err := c.sampleCtrlMessages(p2pmsg.CtrlMsgIHave, controlMessage, sampleSize)
		if err != nil {
			return fmt.Errorf("failed to sample ihave messages: %w", err)
		}
		err = c.validateTopicsSample(from, validationConfig, controlMessage, activeClusterIDS, sampleSize)
		if err != nil {
			lg.Warn().
				Err(err).
				Bool(logging.KeySuspicious, true).
				Msg("topic validation pre-processing failed rejecting rpc control message")
			disErr := c.distributor.Distribute(p2p.NewInvalidControlMessageNotification(from, validationConfig.ControlMsg, count, err))
			if disErr != nil {
				lg.Error().
					Err(disErr).
					Bool(logging.KeySuspicious, true).
					Msg("failed to distribute invalid control message notification")
				return disErr
			}
			return err
		}
	}

	// pre-processing validation passed, perform ihave sampling again
	// to randomize async validation to avoid data race that can occur when
	// performing the sampling asynchronously.
	// for iHave control message topic validation we only validate a random subset of the messages
	err := c.sampleCtrlMessages(p2pmsg.CtrlMsgIHave, controlMessage, sampleSize)
	if err != nil {
		return fmt.Errorf("failed to sample ihave messages: %w", err)
	}
	return nil
}

// sampleCtrlMessages performs sampling on the specified control message that will randomize
// the items in the control message slice up to index sampleSize-1.
func (c *ControlMsgValidationInspector) sampleCtrlMessages(ctrlMsgType p2pmsg.ControlMessageType, ctrlMsg *pubsub_pb.ControlMessage, sampleSize uint) error {
	switch ctrlMsgType {
	case p2pmsg.CtrlMsgIHave:
		iHaves := ctrlMsg.GetIhave()
		swap := func(i, j uint) {
			iHaves[i], iHaves[j] = iHaves[j], iHaves[i]
		}
		err := flowrand.Samples(uint(len(iHaves)), sampleSize, swap)
		if err != nil {
			return fmt.Errorf("failed to get random sample of ihave control messages: %w", err)
		}
	}
	return nil
}

// processInspectMsgReq func used by component workers to perform further inspection of control messages that will check if the messages are rate limited
// and ensure all topic IDS are valid when the amount of messages is above the configured safety threshold.
func (c *ControlMsgValidationInspector) processInspectMsgReq(req *InspectMsgRequest) error {
	c.metrics.AsyncProcessingStarted(req.validationConfig.ControlMsg.String())
	start := time.Now()
	defer func() {
		c.metrics.AsyncProcessingFinished(req.validationConfig.ControlMsg.String(), time.Since(start))
	}()

	count := c.getCtrlMsgCount(req.validationConfig.ControlMsg, req.ctrlMsg)
	lg := c.logger.With().
		Str("peer_id", req.Peer.String()).
		Str("ctrl_msg_type", string(req.validationConfig.ControlMsg)).
		Uint64("ctrl_msg_count", count).Logger()

	var validationErr error
	switch {
	case !c.rateLimiters[req.validationConfig.ControlMsg].Allow(req.Peer, int(count)): // check if Peer RPC messages are rate limited
		validationErr = NewRateLimitedControlMsgErr(req.validationConfig.ControlMsg)
	case count > req.validationConfig.SafetyThreshold:
		// check if Peer RPC messages Count greater than safety threshold further inspect each message individually
		validationErr = c.validateTopics(req.Peer, req.validationConfig, req.ctrlMsg)
	default:
		lg.Trace().
			Uint64("hard_threshold", req.validationConfig.HardThreshold).
			Uint64("safety_threshold", req.validationConfig.SafetyThreshold).
			Msg(fmt.Sprintf("control message %s inspection passed %d is below configured safety threshold", req.validationConfig.ControlMsg, count))
		return nil
	}
	if validationErr != nil {
		lg.Error().
			Err(validationErr).
			Bool(logging.KeySuspicious, true).
			Msg("rpc control message async inspection failed")
		err := c.distributor.Distribute(p2p.NewInvalidControlMessageNotification(req.Peer, req.validationConfig.ControlMsg, count, validationErr))
		if err != nil {
			lg.Error().
				Err(err).
				Bool(logging.KeySuspicious, true).
				Msg("failed to distribute invalid control message notification")
		}
	}
	return nil
}

// getCtrlMsgCount returns the amount of specified control message type in the rpc ControlMessage.
func (c *ControlMsgValidationInspector) getCtrlMsgCount(ctrlMsgType p2pmsg.ControlMessageType, ctrlMsg *pubsub_pb.ControlMessage) uint64 {
	switch ctrlMsgType {
	case p2pmsg.CtrlMsgGraft:
		return uint64(len(ctrlMsg.GetGraft()))
	case p2pmsg.CtrlMsgPrune:
		return uint64(len(ctrlMsg.GetPrune()))
	case p2pmsg.CtrlMsgIHave:
		return uint64(len(ctrlMsg.GetIhave()))
	default:
		return 0
	}
}

// validateTopics ensures all topics in the specified control message are valid flow topic/channel and no duplicate topics exist.
// Expected error returns during normal operations:
//   - channels.InvalidTopicErr: if topic is invalid.
//   - ErrDuplicateTopic: if a duplicate topic ID is encountered.
func (c *ControlMsgValidationInspector) validateTopics(from peer.ID, validationConfig *p2pconf.CtrlMsgValidationConfig, ctrlMsg *pubsub_pb.ControlMessage) error {
	activeClusterIDS := c.tracker.GetActiveClusterIds()
	switch validationConfig.ControlMsg {
	case p2pmsg.CtrlMsgGraft:
		return c.validateGrafts(from, ctrlMsg, activeClusterIDS)
	case p2pmsg.CtrlMsgPrune:
		return c.validatePrunes(from, ctrlMsg, activeClusterIDS)
	case p2pmsg.CtrlMsgIHave:
		return c.validateIhaves(from, validationConfig, ctrlMsg, activeClusterIDS)
	default:
		// sanity check
		// This should never happen validateTopics is only used to validate GRAFT and PRUNE control message types
		// if any other control message type is encountered here this indicates invalid state irrecoverable error.
		c.logger.Fatal().Msg(fmt.Sprintf("encountered invalid control message type in validate topics expected %s, %s or %s got %s", p2pmsg.CtrlMsgGraft, p2pmsg.CtrlMsgPrune, p2pmsg.CtrlMsgIHave, validationConfig.ControlMsg))
	}
	return nil
}

// validateGrafts performs topic validation on all grafts in the control message using the provided validateTopic func while tracking duplicates.
func (c *ControlMsgValidationInspector) validateGrafts(from peer.ID, ctrlMsg *pubsub_pb.ControlMessage, activeClusterIDS flow.ChainIDList) error {
	tracker := make(duplicateTopicTracker)
	for _, graft := range ctrlMsg.GetGraft() {
		topic := channels.Topic(graft.GetTopicID())
		if tracker.isDuplicate(topic) {
			return NewDuplicateTopicErr(topic)
		}
		tracker.set(topic)
		err := c.validateTopic(from, topic, activeClusterIDS)
		if err != nil {
			return err
		}
	}
	return nil
}

// validatePrunes performs topic validation on all prunes in the control message using the provided validateTopic func while tracking duplicates.
func (c *ControlMsgValidationInspector) validatePrunes(from peer.ID, ctrlMsg *pubsub_pb.ControlMessage, activeClusterIDS flow.ChainIDList) error {
	tracker := make(duplicateTopicTracker)
	for _, prune := range ctrlMsg.GetPrune() {
		topic := channels.Topic(prune.GetTopicID())
		if tracker.isDuplicate(topic) {
			return NewDuplicateTopicErr(topic)
		}
		tracker.set(topic)
		err := c.validateTopic(from, topic, activeClusterIDS)
		if err != nil {
			return err
		}
	}
	return nil
}

// validateIhaves performs topic validation on all ihaves in the control message using the provided validateTopic func while tracking duplicates.
func (c *ControlMsgValidationInspector) validateIhaves(from peer.ID, validationConfig *p2pconf.CtrlMsgValidationConfig, ctrlMsg *pubsub_pb.ControlMessage, activeClusterIDS flow.ChainIDList) error {
	sampleSize := util.SampleN(len(ctrlMsg.GetIhave()), c.config.IHaveInspectionMaxSampleSize, c.config.IHaveAsyncInspectSampleSizePercentage)
	return c.validateTopicsSample(from, validationConfig, ctrlMsg, activeClusterIDS, sampleSize)
}

// validateTopicsSample samples a subset of topics from the specified control message and ensures the sample contains only valid flow topic/channel and no duplicate topics exist.
// Sample size ensures liveness of the network when validating messages with no upper bound on the amount of messages that may be received.
// All errors returned from this function can be considered benign.
func (c *ControlMsgValidationInspector) validateTopicsSample(from peer.ID, validationConfig *p2pconf.CtrlMsgValidationConfig, ctrlMsg *pubsub_pb.ControlMessage, activeClusterIDS flow.ChainIDList, sampleSize uint) error {
	tracker := make(duplicateTopicTracker)
	switch validationConfig.ControlMsg {
	case p2pmsg.CtrlMsgIHave:
		for i := uint(0); i < sampleSize; i++ {
			topic := channels.Topic(ctrlMsg.Ihave[i].GetTopicID())
			if tracker.isDuplicate(topic) {
				return NewDuplicateTopicErr(topic)
			}
			tracker.set(topic)
			err := c.validateTopic(from, topic, activeClusterIDS)
			if err != nil {
				return err
			}
		}
	default:
		// sanity check
		// This should never happen validateTopicsSample is only used to validate IHAVE control message types
		// if any other control message type is encountered here this indicates invalid state irrecoverable error.
		c.logger.Fatal().Msg(fmt.Sprintf("encountered invalid control message type in validate topics sample expected %s got %s", p2pmsg.CtrlMsgIHave, validationConfig.ControlMsg))
	}
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
