package validation

import (
	"fmt"
	"sync"

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
	"github.com/onflow/flow-go/network/p2p/inspector/internal"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// DefaultNumberOfWorkers default number of workers for the inspector component.
	DefaultNumberOfWorkers = 5
	// DefaultControlMsgValidationInspectorQueueCacheSize is the default size of the inspect message queue.
	DefaultControlMsgValidationInspectorQueueCacheSize = 100
	// rpcInspectorComponentName the rpc inspector component name.
	rpcInspectorComponentName = "gossipsub_rpc_validation_inspector"
)

// InspectMsgRequest represents a short digest of an RPC control message. It is used for further message inspection by component workers.
type InspectMsgRequest struct {
	// Nonce adds random value so that when msg req is stored on hero store a unique ID can be created from the struct fields.
	Nonce []byte
	// Peer sender of the message.
	Peer peer.ID
	// CtrlMsg the control message that will be inspected.
	ctrlMsg          *pubsub_pb.ControlMessage
	validationConfig *CtrlMsgValidationConfig
}

// ControlMsgValidationInspectorConfig validation configuration for each type of RPC control message.
type ControlMsgValidationInspectorConfig struct {
	// NumberOfWorkers number of component workers to start for processing RPC messages.
	NumberOfWorkers int
	// InspectMsgStoreOpts options used to configure the underlying herocache message store.
	InspectMsgStoreOpts []queue.HeroStoreConfigOption
	// GraftValidationCfg validation configuration for GRAFT control messages.
	GraftValidationCfg *CtrlMsgValidationConfig
	// PruneValidationCfg validation configuration for PRUNE control messages.
	PruneValidationCfg *CtrlMsgValidationConfig
}

// getCtrlMsgValidationConfig returns the CtrlMsgValidationConfig for the specified p2p.ControlMessageType.
func (conf *ControlMsgValidationInspectorConfig) getCtrlMsgValidationConfig(controlMsg p2p.ControlMessageType) (*CtrlMsgValidationConfig, bool) {
	switch controlMsg {
	case p2p.CtrlMsgGraft:
		return conf.GraftValidationCfg, true
	case p2p.CtrlMsgPrune:
		return conf.PruneValidationCfg, true
	default:
		return nil, false
	}
}

// allCtrlMsgValidationConfig returns all control message validation configs in a list.
func (conf *ControlMsgValidationInspectorConfig) allCtrlMsgValidationConfig() CtrlMsgValidationConfigs {
	return CtrlMsgValidationConfigs{conf.GraftValidationCfg, conf.PruneValidationCfg}
}

// ControlMsgValidationInspector RPC message inspector that inspects control messages and performs some validation on them,
// when some validation rule is broken feedback is given via the Peer scoring notifier.
type ControlMsgValidationInspector struct {
	component.Component
	logger  zerolog.Logger
	sporkID flow.Identifier
	// lock RW mutex used to synchronize access to the  clusterIDSProvider.
	lock sync.RWMutex
	// clusterIDSProvider the cluster IDS providers provides active cluster IDs for cluster Topic validation. The
	// clusterIDSProvider must be configured for LN nodes to validate control message with cluster prefixed topics.
	clusterIDSProvider module.ClusterIDSProvider
	// config control message validation configurations.
	config *ControlMsgValidationInspectorConfig
	// distributor used to disseminate invalid RPC message notifications.
	distributor p2p.GossipSubInspectorNotificationDistributor
	// workerPool queue that stores *InspectMsgRequest that will be processed by component workers.
	workerPool *worker.Pool[*InspectMsgRequest]
}

var _ component.Component = (*ControlMsgValidationInspector)(nil)
var _ p2p.GossipSubRPCInspector = (*ControlMsgValidationInspector)(nil)

// NewInspectMsgRequest returns a new *InspectMsgRequest.
func NewInspectMsgRequest(from peer.ID, validationConfig *CtrlMsgValidationConfig, ctrlMsg *pubsub_pb.ControlMessage) (*InspectMsgRequest, error) {
	nonce, err := internal.Nonce()
	if err != nil {
		return nil, fmt.Errorf("failed to get inspect message request nonce: %w", err)
	}
	return &InspectMsgRequest{Nonce: nonce, Peer: from, validationConfig: validationConfig, ctrlMsg: ctrlMsg}, nil
}

// NewControlMsgValidationInspector returns new ControlMsgValidationInspector
func NewControlMsgValidationInspector(
	logger zerolog.Logger,
	sporkID flow.Identifier,
	config *ControlMsgValidationInspectorConfig,
	distributor p2p.GossipSubInspectorNotificationDistributor,
) *ControlMsgValidationInspector {
	lg := logger.With().Str("component", "gossip_sub_rpc_validation_inspector").Logger()
	c := &ControlMsgValidationInspector{
		logger:      lg,
		sporkID:     sporkID,
		config:      config,
		distributor: distributor,
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
	return c
}

// Inspect inspects the rpc received and returns an error if any validation rule is broken.
// For each control message type an initial inspection is done synchronously to check the amount
// of messages in the control message. Further inspection is done asynchronously to check rate limits
// and validate topic IDS each control message if initial validation is passed.
// All errors returned from this function can be considered benign.
// errors returned:
//
//	ErrDiscardThreshold - if the message count for the control message type exceeds the discard threshold.
//
// This func returns an exception in case of unexpected bug or state corruption the violation distributor
// fails to distribute invalid control message notification or a new inspect message request can't be created.
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

// SetClusterIDSProvider sets the cluster IDs provider that is used to provider cluster ID information
// about active clusters for collection nodes. This method should only be called once, and subsequent calls
// will be a no-op.
func (c *ControlMsgValidationInspector) SetClusterIDSProvider(provider module.ClusterIDSProvider) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.clusterIDSProvider == nil {
		c.clusterIDSProvider = provider
	}
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
		err := c.distributor.DistributeInvalidControlMessageNotification(p2p.NewInvalidControlMessageNotification(from, validationConfig.ControlMsg, count, discardThresholdErr))
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
		validationErr = c.validateTopics(req.validationConfig.ControlMsg, req.ctrlMsg)
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
		err := c.distributor.DistributeInvalidControlMessageNotification(p2p.NewInvalidControlMessageNotification(req.Peer, req.validationConfig.ControlMsg, count, validationErr))
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
func (c *ControlMsgValidationInspector) validateTopics(ctrlMsgType p2p.ControlMessageType, ctrlMsg *pubsub_pb.ControlMessage) error {
	seen := make(map[channels.Topic]struct{})
	validateTopic := func(topic channels.Topic) error {
		if _, ok := seen[topic]; ok {
			return NewIDuplicateTopicErr(topic)
		}
		seen[topic] = struct{}{}
		err := c.validateTopic(topic, ctrlMsgType)
		if err != nil {
			return err
		}
		return nil
	}
	switch ctrlMsgType {
	case p2p.CtrlMsgGraft:
		for _, graft := range ctrlMsg.GetGraft() {
			topic := channels.Topic(graft.GetTopicID())
			err := validateTopic(topic)
			if err != nil {
				return err
			}
		}
	case p2p.CtrlMsgPrune:
		for _, prune := range ctrlMsg.GetPrune() {
			topic := channels.Topic(prune.GetTopicID())
			err := validateTopic(topic)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// validateTopic ensures the topic is a valid flow topic/channel.
// Expected error returns during normal operations:
//   - channels.ErrInvalidTopic: if topic is invalid.
//
// This func returns an exception in case of unexpected bug or state corruption if cluster prefixed topic validation
// fails due to unexpected error returned when getting the active cluster IDS.
func (c *ControlMsgValidationInspector) validateTopic(topic channels.Topic, ctrlMsgType p2p.ControlMessageType) error {
	channel, ok := channels.ChannelFromTopic(topic)
	if !ok {
		return channels.NewInvalidTopicErr(topic, fmt.Errorf("failed to get channel from topic"))
	}

	// handle cluster prefixed topics
	if channels.IsClusterChannel(channel) {
		return c.validateClusterPrefixedTopic(topic, ctrlMsgType)
	}

	// non cluster prefixed topic validation
	err := channels.IsValidFlowTopic(topic, c.sporkID)
	if err != nil {
		return err
	}
	return nil
}

// validateClusterPrefixedTopic validates cluster prefixed topics.
// Expected error returns during normal operations:
//   - channels.ErrInvalidTopic: if topic is invalid.
//
// This func returns an exception in case of unexpected bug or state corruption if cluster prefixed topic validation
// fails due to unexpected error returned when getting the active cluster IDS.
func (c *ControlMsgValidationInspector) validateClusterPrefixedTopic(topic channels.Topic, ctrlMsgType p2p.ControlMessageType) error {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if c.clusterIDSProvider == nil {
		c.logger.Warn().
			Str("topic", topic.String()).
			Str("ctrl_msg_type", string(ctrlMsgType)).
			Msg("failed to validate control message with cluster pre-fixed topic cluster ids provider is not set")
		return nil
	}
	activeClusterIDS, err := c.clusterIDSProvider.ActiveClusterIDS()
	if err != nil {
		return fmt.Errorf("failed to get active cluster IDS: %w", err)
	}

	err = channels.IsValidFlowClusterTopic(topic, activeClusterIDS)
	if err != nil {
		return err
	}
	return nil
}
