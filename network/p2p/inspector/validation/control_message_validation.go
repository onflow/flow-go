package validation

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	DefaultNumberOfWorkers = 5
)

// inspectMsgReq represents a short digest of an RPC control message. It is used for further message inspection by component workers.
type inspectMsgReq struct {
	peer             peer.ID
	validationConfig *CtrlMsgValidationConfig
	topicIDS         []string
	count            uint64
}

// ControlMsgValidationInspectorConfig validation configuration for each type of RPC control message.
type ControlMsgValidationInspectorConfig struct {
	// NumberOfWorkers number of component workers to start for processing RPC messages.
	NumberOfWorkers int
	// GraftValidationCfg validation configuration for GRAFT control messages.
	GraftValidationCfg *CtrlMsgValidationConfig
	// PruneValidationCfg validation configuration for PRUNE control messages.
	PruneValidationCfg *CtrlMsgValidationConfig
}

func (conf *ControlMsgValidationInspectorConfig) config(controlMsg p2p.ControlMessageType) (*CtrlMsgValidationConfig, bool) {
	switch controlMsg {
	case p2p.CtrlMsgGraft:
		return conf.GraftValidationCfg, true
	case p2p.CtrlMsgPrune:
		return conf.PruneValidationCfg, true
	default:
		return nil, false
	}
}

// configs returns all control message validation configs in a list.
func (conf *ControlMsgValidationInspectorConfig) configs() CtrlMsgValidationConfigs {
	return CtrlMsgValidationConfigs{conf.GraftValidationCfg, conf.PruneValidationCfg}
}

// ControlMsgValidationInspector RPC message inspector that inspects control messages and performs some validation on them,
// when some validation rule is broken feedback is given via the peer scoring notifier.
type ControlMsgValidationInspector struct {
	component.Component
	logger          zerolog.Logger
	inspectMessageQ chan *inspectMsgReq
	// validationConfig control message validation configurations.
	validationConfig *ControlMsgValidationInspectorConfig
	// distributor used to disseminate invalid RPC message notifications.
	distributor p2p.GossipSubInspectorNotificationDistributor
}

var _ component.Component = (*ControlMsgValidationInspector)(nil)

// NewControlMsgValidationInspector returns new ControlMsgValidationInspector
func NewControlMsgValidationInspector(logger zerolog.Logger, validationConfig *ControlMsgValidationInspectorConfig, distributor p2p.GossipSubInspectorNotificationDistributor) *ControlMsgValidationInspector {
	c := &ControlMsgValidationInspector{
		logger:           logger.With().Str("component", "gossip-sub-rpc-validation-inspector").Logger(),
		inspectMessageQ:  make(chan *inspectMsgReq),
		validationConfig: validationConfig,
		distributor:      distributor,
	}
	builder := component.NewComponentManagerBuilder()
	// start rate limiters cleanup loop in workers
	for _, config := range c.validationConfig.configs() {
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			config.RateLimiter.CleanupLoop(ctx)
		})
	}
	for i := 0; i < c.validationConfig.NumberOfWorkers; i++ {
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			c.inspectMessageLoop(ctx)
		})
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

	err := c.inspect(from, p2p.CtrlMsgGraft, control)
	if err != nil {
		return fmt.Errorf("validation failed for control message %s: %w", p2p.CtrlMsgGraft, err)
	}

	err = c.inspect(from, p2p.CtrlMsgPrune, control)
	if err != nil {
		return fmt.Errorf("validation failed for control message %s: %w", p2p.CtrlMsgPrune, err)
	}

	return nil
}

// inspect performs initial inspection of RPC control message and queues up message for further inspection if required.
// All errors returned from this function can be considered benign.
func (c *ControlMsgValidationInspector) inspect(from peer.ID, ctrlMsgType p2p.ControlMessageType, ctrlMsg *pubsub_pb.ControlMessage) error {
	validationConfig, ok := c.validationConfig.config(ctrlMsgType)
	if !ok {
		return fmt.Errorf("failed to get validation configuration for control message %s", ctrlMsg)
	}
	count, topicIDS := c.getCtrlMsgData(ctrlMsgType, ctrlMsg)
	// if count greater than upper threshold drop message and penalize
	if count > validationConfig.UpperThreshold {
		upperThresholdErr := NewUpperThresholdErr(validationConfig.ControlMsg, count, validationConfig.UpperThreshold)
		c.logger.Warn().
			Err(upperThresholdErr).
			Bool(logging.KeySuspicious, true).
			Msg("rejecting RPC message")

		err := c.distributor.DistributeInvalidControlMessageNotification(p2p.NewInvalidControlMessageNotification(from, ctrlMsgType, count, upperThresholdErr))
		if err != nil {
			return fmt.Errorf("failed to distribute invalid control message notification: %w", err)
		}
		return upperThresholdErr
	}
	// queue further async inspection
	c.requestMsgInspection(&inspectMsgReq{peer: from, validationConfig: validationConfig, topicIDS: topicIDS, count: count})
	return nil
}

// processInspectMsgReq func used by component workers to perform further inspection of control messages that will check if the messages are rate limited
// and ensure all topic IDS are valid when the amount of messages is above the configured safety threshold.
func (c *ControlMsgValidationInspector) processInspectMsgReq(req *inspectMsgReq) {
	lg := c.logger.With().
		Uint64("count", req.count).
		Str("control-message", string(req.validationConfig.ControlMsg)).Logger()
	var validationErr error
	switch {
	case !req.validationConfig.RateLimiter.Allow(req.peer, int(req.count)): // check if peer RPC messages are rate limited
		validationErr = NewRateLimitedControlMsgErr(req.validationConfig.ControlMsg)
	case req.count > req.validationConfig.SafetyThreshold: // check if peer RPC messages count greater than safety threshold further inspect each message individually
		validationErr = c.validateTopics(req.validationConfig.ControlMsg, req.topicIDS)
	default:
		lg.Info().
			Msg(fmt.Sprintf("skipping RPC control message %s inspection validation message count %d below safety threshold", req.validationConfig.ControlMsg, req.count))
	}
	if validationErr != nil {
		lg.Error().
			Err(validationErr).
			Bool(logging.KeySuspicious, true).
			Msg(fmt.Sprintf("rpc control message async inspection failed"))
		err := c.distributor.DistributeInvalidControlMessageNotification(p2p.NewInvalidControlMessageNotification(req.peer, req.validationConfig.ControlMsg, req.count, validationErr))
		if err != nil {
			lg.Error().
				Err(err).
				Msg("failed to distribute invalid control message notification")
		}
	}
}

// requestMsgInspection queues up an inspect message request.
func (c *ControlMsgValidationInspector) requestMsgInspection(req *inspectMsgReq) {
	c.inspectMessageQ <- req
}

// inspectMessageLoop callback used by component workers to process inspect message request
// from the validation inspector whenever further inspection of an RPC message is needed.
func (c *ControlMsgValidationInspector) inspectMessageLoop(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-ctx.Done():
			return
		case request := <-c.inspectMessageQ:
			c.processInspectMsgReq(request)
		}
	}
}

// getCtrlMsgData returns the amount of specified control message type in the rpc ControlMessage as well as the topic ID for each message.
func (c *ControlMsgValidationInspector) getCtrlMsgData(ctrlMsgType p2p.ControlMessageType, ctrlMsg *pubsub_pb.ControlMessage) (uint64, []string) {
	topicIDS := make([]string, 0)
	count := 0
	switch ctrlMsgType {
	case p2p.CtrlMsgGraft:
		grafts := ctrlMsg.GetGraft()
		for _, graft := range grafts {
			topicIDS = append(topicIDS, graft.GetTopicID())
		}
		count = len(grafts)
	case p2p.CtrlMsgPrune:
		prunes := ctrlMsg.GetPrune()
		for _, prune := range prunes {
			topicIDS = append(topicIDS, prune.GetTopicID())
		}
		count = len(prunes)
	}

	return uint64(count), topicIDS
}

// validateTopics ensures the topic is a valid flow topic/channel and the node has a subscription to that topic.
// All errors returned from this function can be considered benign.
func (c *ControlMsgValidationInspector) validateTopics(ctrlMsg p2p.ControlMessageType, topics []string) error {
	var errs *multierror.Error
	for _, t := range topics {
		topic := channels.Topic(t)
		channel, ok := channels.ChannelFromTopic(topic)
		if !ok {
			errs = multierror.Append(errs, NewMalformedTopicErr(ctrlMsg, topic))
			continue
		}

		if !channels.ChannelExists(channel) {
			errs = multierror.Append(errs, NewUnknownTopicChannelErr(ctrlMsg, topic))
		}
	}
	return errs.ErrorOrNil()
}
