package validation

import (
	"fmt"
	"math/rand"

	"github.com/hashicorp/go-multierror"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/common/worker"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// DefaultNumberOfWorkers default number of workers for the inspector component.
	DefaultNumberOfWorkers = 5
	// DefaultControlMsgValidationInspectorQueueCacheSize is the default size of the inspect message queue.
	DefaultControlMsgValidationInspectorQueueCacheSize = 100
)

// InspectMsgReq represents a short digest of an RPC control message. It is used for further message inspection by component workers.
type InspectMsgReq struct {
	// Nonce adds random value so that when msg req is stored on hero store a unique ID can be created from the struct fields.
	Nonce uint64
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
	logger zerolog.Logger
	// config control message validation configurations.
	config *ControlMsgValidationInspectorConfig
	// distributor used to disseminate invalid RPC message notifications.
	distributor p2p.GossipSubInspectorNotificationDistributor
	// workerPool queue that stores *InspectMsgReq that will be processed by component workers.
	workerPool *worker.Pool[*InspectMsgReq]
}

var _ component.Component = (*ControlMsgValidationInspector)(nil)

// NewInspectMsgReq returns a new *InspectMsgReq.
func NewInspectMsgReq(from peer.ID, validationConfig *CtrlMsgValidationConfig, ctrlMsg *pubsub_pb.ControlMessage) *InspectMsgReq {
	return &InspectMsgReq{Nonce: rand.Uint64(), Peer: from, validationConfig: validationConfig, ctrlMsg: ctrlMsg}
}

// NewControlMsgValidationInspector returns new ControlMsgValidationInspector
func NewControlMsgValidationInspector(
	logger zerolog.Logger,
	config *ControlMsgValidationInspectorConfig,
	distributor p2p.GossipSubInspectorNotificationDistributor,
) *ControlMsgValidationInspector {
	lg := logger.With().Str("component", "gossip_sub_rpc_validation_inspector").Logger()
	c := &ControlMsgValidationInspector{
		logger:      lg,
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
	pool := worker.NewWorkerPoolBuilder[*InspectMsgReq](lg, store, c.processInspectMsgReq).Build()

	c.workerPool = pool

	builder := component.NewComponentManagerBuilder()
	// start rate limiters cleanup loop in workers
	for _, conf := range c.config.allCtrlMsgValidationConfig() {
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			conf.RateLimiter.CleanupLoop(ctx)
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
		count := c.getCtrlMsgCount(ctrlMsgType, control)
		// if Count greater than upper threshold drop message and penalize
		if count > validationConfig.UpperThreshold {
			upperThresholdErr := NewUpperThresholdErr(validationConfig.ControlMsg, count, validationConfig.UpperThreshold)
			lg.Warn().
				Err(upperThresholdErr).
				Uint64("ctrl_msg_count", count).
				Uint64("upper_threshold", upperThresholdErr.upperThreshold).
				Bool(logging.KeySuspicious, true).
				Msg("rejecting rpc message")
			err := c.distributor.DistributeInvalidControlMessageNotification(p2p.NewInvalidControlMessageNotification(from, ctrlMsgType, count, upperThresholdErr))
			if err != nil {
				lg.Error().
					Err(err).
					Bool(logging.KeySuspicious, true).
					Msg("failed to distribute invalid control message notification")
				return err
			}
			return upperThresholdErr
		}

		// queue further async inspection
		c.requestMsgInspection(NewInspectMsgReq(from, validationConfig, control))
	}

	return nil
}

// processInspectMsgReq func used by component workers to perform further inspection of control messages that will check if the messages are rate limited
// and ensure all topic IDS are valid when the amount of messages is above the configured safety threshold.
func (c *ControlMsgValidationInspector) processInspectMsgReq(req *InspectMsgReq) error {
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
		validationErr = c.validateCtrlMsgTopics(req.validationConfig.ControlMsg, req.ctrlMsg)
	default:
		lg.Trace().
			Uint64("upper_threshold", req.validationConfig.UpperThreshold).
			Uint64("safety_threshold", req.validationConfig.SafetyThreshold).
			Msg(fmt.Sprintf("control message %s inspection passed %d is below configured safety threshold", req.validationConfig.ControlMsg, count))
		return nil
	}
	if validationErr != nil {
		lg.Error().
			Err(validationErr).
			Bool(logging.KeySuspicious, true).
			Msg(fmt.Sprintf("rpc control message async inspection failed"))
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

// requestMsgInspection queues up an inspect message request.
func (c *ControlMsgValidationInspector) requestMsgInspection(req *InspectMsgReq) {
	c.workerPool.Submit(req)
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

// validateCtrlMsgTopics ensures all topics in the specified control message are valid flow topic/channel.
// All errors returned from this function can be considered benign.
func (c *ControlMsgValidationInspector) validateCtrlMsgTopics(ctrlMsgType p2p.ControlMessageType, ctrlMsg *pubsub_pb.ControlMessage) error {
	topicIDS := make([]string, 0)
	switch ctrlMsgType {
	case p2p.CtrlMsgGraft:
		for _, graft := range ctrlMsg.GetGraft() {
			topicIDS = append(topicIDS, graft.GetTopicID())
		}
	case p2p.CtrlMsgPrune:
		for _, prune := range ctrlMsg.GetPrune() {
			topicIDS = append(topicIDS, prune.GetTopicID())
		}
	}
	return c.validateTopics(ctrlMsgType, topicIDS)
}

// validateTopics ensures all topics are valid flow topic/channel.
// All errors returned from this function can be considered benign.
func (c *ControlMsgValidationInspector) validateTopics(ctrlMsgType p2p.ControlMessageType, topicIDS []string) error {
	var errs *multierror.Error
	for _, t := range topicIDS {
		topic := channels.Topic(t)
		channel, ok := channels.ChannelFromTopic(topic)
		if !ok {
			errs = multierror.Append(errs, NewMalformedTopicErr(ctrlMsgType, topic))
			continue
		}

		if !channels.ChannelExists(channel) {
			errs = multierror.Append(errs, NewUnknownTopicChannelErr(ctrlMsgType, topic))
		}
	}
	return errs.ErrorOrNil()
}
