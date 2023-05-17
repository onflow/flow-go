package validation

import (
	"fmt"

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
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/internal/cache"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/utils/logging"
)

// ControlMsgValidationInspector RPC message inspector that inspects control messages and performs some validation on them,
// when some validation rule is broken feedback is given via the Peer scoring notifier.
type ControlMsgValidationInspector struct {
	component.Component
	events.Noop
	logger  zerolog.Logger
	sporkID flow.Identifier
	// config control message validation configurations.
	config *ControlMsgValidationInspectorConfig
	// distributor used to disseminate invalid RPC message notifications.
	distributor p2p.GossipSubInspectorNotifDistributor
	// workerPool queue that stores *InspectMsgRequest that will be processed by component workers.
	workerPool *worker.Pool[*InspectMsgRequest]
	// clusterPrefixTopicsReceivedTracker is a map that associates the hash of a peer's ID with the
	// number of cluster-prefix topic control messages received from that peer. It helps in tracking
	// and managing the rate of incoming control messages from each peer, ensuring that the system
	// stays performant and resilient against potential spam or abuse.
	// The counter is incremented in the following scenarios:
	// 1. The cluster prefix topic is received while the inspector waits for the cluster IDs provider to be set (this can happen during the startup or epoch transitions).
	// 2. The node sends a cluster prefix topic where the cluster prefix does not match any of the active cluster IDs.
	// In such cases, the inspector will allow a configured number of these messages from the corresponding peer.
	clusterPrefixTopicsReceivedTracker *cache.ClusterPrefixTopicsReceivedTracker
	idProvider                         module.IdentityProvider
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
func NewControlMsgValidationInspector(logger zerolog.Logger, sporkID flow.Identifier, config *ControlMsgValidationInspectorConfig, distributor p2p.GossipSubInspectorNotifDistributor, clusterPrefixedCacheCollector module.HeroCacheMetrics, idProvider module.IdentityProvider) (*ControlMsgValidationInspector, error) {
	lg := logger.With().Str("component", "gossip_sub_rpc_validation_inspector").Logger()

	tracker, err := cache.NewClusterPrefixTopicsReceivedTracker(logger, config.ClusterPrefixedTopicsReceivedCacheSize, clusterPrefixedCacheCollector, config.ClusterPrefixedTopicsReceivedCacheDecay)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster prefix topics received tracker")
	}

	c := &ControlMsgValidationInspector{
		logger:                             lg,
		sporkID:                            sporkID,
		config:                             config,
		distributor:                        distributor,
		clusterPrefixTopicsReceivedTracker: tracker,
		idProvider:                         idProvider,
	}

	cfg := &queue.HeroStoreConfig{
		SizeLimit: DefaultControlMsgValidationInspectorQueueCacheSize,
		Collector: metrics.NewNoopCollector(),
	}

	for _, opt := range config.InspectMsgStoreOpts {
		opt(cfg)
	}

	store := queue.NewHeroStore(cfg.SizeLimit, logger, cfg.Collector)
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
	for _, conf := range c.config.allCtrlMsgValidationConfig() {
		validationConfig := conf
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			validationConfig.RateLimiter.Start(ctx)
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
	for _, ctrlMsgType := range p2p.ControlMessageTypes() {
		lg := c.logger.With().
			Str("peer_id", from.String()).
			Str("ctrl_msg_type", string(ctrlMsgType)).Logger()
		validationConfig, ok := c.config.getCtrlMsgValidationConfig(ctrlMsgType)
		if !ok {
			lg.Trace().Msg("validation configuration for control type does not exists skipping")
			continue
		}

		// mandatory blocking pre-processing of RPC to check discard threshold.
		err := c.blockingPreprocessingRpc(from, validationConfig, control)
		if err != nil {
			lg.Error().
				Err(err).
				Str("peer_id", from.String()).
				Str("ctrl_msg_type", string(ctrlMsgType)).
				Msg("could not pre-process rpc, aborting")
			return fmt.Errorf("could not pre-process rpc, aborting: %w", err)
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

// ClusterIdsUpdated consumes cluster ID update protocol events.
func (c *ControlMsgValidationInspector) ClusterIdsUpdated(clusterIDList flow.ChainIDList) {
	c.clusterPrefixTopicsReceivedTracker.StoreActiveClusterIds(clusterIDList)
}

// blockingPreprocessingRpc ensures the RPC control message count does not exceed the configured discard threshold.
// Expected error returns during normal operations:
//   - ErrDiscardThreshold: if control message count exceeds the configured discard threshold.
func (c *ControlMsgValidationInspector) blockingPreprocessingRpc(from peer.ID, validationConfig *CtrlMsgValidationConfig, controlMessage *pubsub_pb.ControlMessage) error {
	lg := c.logger.With().
		Str("peer_id", from.String()).
		Str("ctrl_msg_type", string(validationConfig.ControlMsg)).Logger()

	count := c.getCtrlMsgCount(validationConfig.ControlMsg, controlMessage)
	// if Count greater than discard threshold drop message and penalize
	if count > validationConfig.DiscardThreshold {
		discardThresholdErr := NewDiscardThresholdErr(validationConfig.ControlMsg, count, validationConfig.DiscardThreshold)
		lg.Warn().
			Err(discardThresholdErr).
			Uint64("ctrl_msg_count", count).
			Uint64("upper_threshold", discardThresholdErr.discardThreshold).
			Bool(logging.KeySuspicious, true).
			Msg("rejecting rpc control message")
		err := c.distributor.Distribute(p2p.NewInvalidControlMessageNotification(from, validationConfig.ControlMsg, count, discardThresholdErr))
		if err != nil {
			lg.Error().
				Err(err).
				Bool(logging.KeySuspicious, true).
				Msg("failed to distribute invalid control message notification")
			return err
		}
		return discardThresholdErr
	}

	return nil
}

// processInspectMsgReq func used by component workers to perform further inspection of control messages that will check if the messages are rate limited
// and ensure all topic IDS are valid when the amount of messages is above the configured safety threshold.
func (c *ControlMsgValidationInspector) processInspectMsgReq(req *InspectMsgRequest) error {
	count := c.getCtrlMsgCount(req.validationConfig.ControlMsg, req.ctrlMsg)
	lg := c.logger.With().
		Str("peer_id", req.Peer.String()).
		Str("ctrl_msg_type", string(req.validationConfig.ControlMsg)).
		Uint64("ctrl_msg_count", count).Logger()
	var validationErr error
	switch {
	case !req.validationConfig.RateLimiter.Allow(req.Peer, int(count)): // check if Peer RPC messages are rate limited
		validationErr = NewRateLimitedControlMsgErr(req.validationConfig.ControlMsg)
	case count > req.validationConfig.SafetyThreshold: // check if Peer RPC messages Count greater than safety threshold further inspect each message individually
		validationErr = c.validateTopics(req.Peer, req.validationConfig.ControlMsg, req.ctrlMsg)
	default:
		lg.Trace().
			Uint64("upper_threshold", req.validationConfig.DiscardThreshold).
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
func (c *ControlMsgValidationInspector) getCtrlMsgCount(ctrlMsgType p2p.ControlMessageType, ctrlMsg *pubsub_pb.ControlMessage) uint64 {
	switch ctrlMsgType {
	case p2p.CtrlMsgGraft:
		return uint64(len(ctrlMsg.GetGraft()))
	case p2p.CtrlMsgPrune:
		return uint64(len(ctrlMsg.GetPrune()))
	default:
		return 0
	}
}

// validateTopics ensures all topics in the specified control message are valid flow topic/channel and no duplicate topics exist.
// Expected error returns during normal operations:
//   - channels.ErrInvalidTopic: if topic is invalid.
//   - ErrDuplicateTopic: if a duplicate topic ID is encountered.
func (c *ControlMsgValidationInspector) validateTopics(from peer.ID, ctrlMsgType p2p.ControlMessageType, ctrlMsg *pubsub_pb.ControlMessage) error {
	activeClusterIDS := c.clusterPrefixTopicsReceivedTracker.GetActiveClusterIds()
	switch ctrlMsgType {
	case p2p.CtrlMsgGraft:
		return c.validateGrafts(from, ctrlMsg, activeClusterIDS)
	case p2p.CtrlMsgPrune:
		return c.validatePrunes(from, ctrlMsg, activeClusterIDS)
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

// validateTopic ensures the topic is a valid flow topic/channel.
// Expected error returns during normal operations:
//   - channels.ErrInvalidTopic: if topic is invalid.
//   - ErrActiveClusterIdsNotSet: if the cluster ID provider is not set.
//   - channels.ErrUnknownClusterID: if the topic contains a cluster ID prefix that is not in the active cluster IDs list.
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
//   - channels.ErrInvalidTopic: if topic is invalid.
//   - channels.ErrUnknownClusterID: if the topic contains a cluster ID prefix that is not in the active cluster IDs list.
//
// In the case where an ErrActiveClusterIdsNotSet or ErrUnknownClusterID is encountered and the cluster prefixed topic received
// tracker for the peer is less than or equal to the configured ClusterPrefixHardThreshold an error will only be logged and not returned.
// At the point where the hard threshold is crossed the error will be returned and the sender will start to be penalized.
func (c *ControlMsgValidationInspector) validateClusterPrefixedTopic(from peer.ID, topic channels.Topic, activeClusterIds flow.ChainIDList) error {
	lg := c.logger.With().
		Str("from", from.String()).
		Logger()
	// reject messages from unstaked nodes for cluster prefixed topics
	identifier, err := c.getFlowIdentifier(from)
	if err != nil {
		return err
	}

	if len(activeClusterIds) == 0 {
		// cluster IDs have not been updated yet
		_, err = c.clusterPrefixTopicsReceivedTracker.Inc(identifier)
		if err != nil {
			return err
		}

		// if the amount of messages received is below our hard threshold log the error and return nil.
		if c.checkClusterPrefixHardThreshold(identifier) {
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
		if channels.IsErrUnknownClusterID(err) {
			// unknown cluster ID error could indicate that a node has fallen
			// behind and needs to catchup increment to topics received cache.
			_, incErr := c.clusterPrefixTopicsReceivedTracker.Inc(identifier)
			if incErr != nil {
				return incErr
			}
			// if the amount of messages received is below our hard threshold log the error and return nil.
			if c.checkClusterPrefixHardThreshold(identifier) {
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
func (c *ControlMsgValidationInspector) checkClusterPrefixHardThreshold(identifier flow.Identifier) bool {
	return c.clusterPrefixTopicsReceivedTracker.Load(identifier) <= c.config.ClusterPrefixHardThreshold
}
