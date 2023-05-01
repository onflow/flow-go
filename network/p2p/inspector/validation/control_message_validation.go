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
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/internal"
	"github.com/onflow/flow-go/utils/logging"
	flowrand "github.com/onflow/flow-go/utils/rand"
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
	// IHaveValidationCfg validation configuration for IHAVE control messages.
	IHaveValidationCfg *CtrlMsgValidationConfig
}

// getCtrlMsgValidationConfig returns the CtrlMsgValidationConfig for the specified p2p.ControlMessageType.
func (conf *ControlMsgValidationInspectorConfig) getCtrlMsgValidationConfig(controlMsg p2p.ControlMessageType) (*CtrlMsgValidationConfig, bool) {
	switch controlMsg {
	case p2p.CtrlMsgGraft:
		return conf.GraftValidationCfg, true
	case p2p.CtrlMsgPrune:
		return conf.PruneValidationCfg, true
	case p2p.CtrlMsgIHave:
		return conf.IHaveValidationCfg, true
	default:
		return nil, false
	}
}

// allCtrlMsgValidationConfig returns all control message validation configs in a list.
func (conf *ControlMsgValidationInspectorConfig) allCtrlMsgValidationConfig() CtrlMsgValidationConfigs {
	return CtrlMsgValidationConfigs{conf.GraftValidationCfg, conf.PruneValidationCfg, conf.IHaveValidationCfg}
}

// ControlMsgValidationInspector RPC message inspector that inspects control messages and performs some validation on them,
// when some validation rule is broken feedback is given via the Peer scoring notifier.
type ControlMsgValidationInspector struct {
	component.Component
	logger  zerolog.Logger
	sporkID flow.Identifier
	metrics module.GossipSubRpcValidationInspectorMetrics
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
	inspectorMetrics module.GossipSubRpcValidationInspectorMetrics,
) *ControlMsgValidationInspector {
	lg := logger.With().Str("component", "gossip_sub_rpc_validation_inspector").Logger()
	c := &ControlMsgValidationInspector{
		logger:      lg,
		sporkID:     sporkID,
		metrics:     inspectorMetrics,
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
//	ErrHardThreshold - if the message count for the control message type exceeds the hard threshold.
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

		switch ctrlMsgType {
		case p2p.CtrlMsgGraft, p2p.CtrlMsgPrune:
			// normal pre-processing
			err := c.blockingPreprocessingRpc(from, validationConfig, control)
			if err != nil {
				lg.Error().
					Err(err).
					Msg("could not pre-process rpc, aborting")
				return fmt.Errorf("could not pre-process rpc, aborting: %w", err)
			}
		case p2p.CtrlMsgIHave:
			// iHave specific pre-processing
			sampleSize := util.SampleN(len(control.GetIhave()), validationConfig.IHaveInspectionMaxSampleSize, validationConfig.IHaveSyncInspectSampleSizePercentage)
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

// blockingPreprocessingRpc generic pre-processing validation func that ensures the RPC control message count does not exceed the configured hard threshold.
func (c *ControlMsgValidationInspector) blockingPreprocessingRpc(from peer.ID, validationConfig *CtrlMsgValidationConfig, controlMessage *pubsub_pb.ControlMessage) error {
	count := c.getCtrlMsgCount(validationConfig.ControlMsg, controlMessage)
	lg := c.logger.With().
		Uint64("ctrl_msg_count", count).
		Str("peer_id", from.String()).
		Str("ctrl_msg_type", string(validationConfig.ControlMsg)).Logger()

	c.metrics.PreProcessingStarted(validationConfig.ControlMsg.String(), uint(count))
	start := time.Now()
	defer func() {
		c.metrics.PreProcessingFinished(validationConfig.ControlMsg.String(), uint(count), time.Since(start))
	}()

	// if Count greater than hard threshold drop message and penalize
	if count > validationConfig.HardThreshold {
		hardThresholdErr := NewHardThresholdErr(validationConfig.ControlMsg, count, validationConfig.HardThreshold)
		lg.Warn().
			Err(hardThresholdErr).
			Uint64("upper_threshold", hardThresholdErr.hardThreshold).
			Bool(logging.KeySuspicious, true).
			Msg("rejecting rpc control message")
		err := c.distributor.DistributeInvalidControlMessageNotification(p2p.NewInvalidControlMessageNotification(from, validationConfig.ControlMsg, count, hardThresholdErr))
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
func (c *ControlMsgValidationInspector) blockingIHaveSamplePreprocessing(from peer.ID, validationConfig *CtrlMsgValidationConfig, controlMessage *pubsub_pb.ControlMessage, sampleSize uint) error {
	c.metrics.PreProcessingStarted(p2p.CtrlMsgIHave.String(), sampleSize)
	start := time.Now()
	defer func() {
		c.metrics.PreProcessingFinished(p2p.CtrlMsgIHave.String(), sampleSize, time.Since(start))
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
func (c *ControlMsgValidationInspector) blockingPreprocessingSampleRpc(from peer.ID, validationConfig *CtrlMsgValidationConfig, controlMessage *pubsub_pb.ControlMessage, sampleSize uint) error {
	count := c.getCtrlMsgCount(validationConfig.ControlMsg, controlMessage)
	lg := c.logger.With().
		Uint64("ctrl_msg_count", count).
		Str("peer_id", from.String()).
		Str("ctrl_msg_type", string(validationConfig.ControlMsg)).Logger()
	// if count greater than hard threshold perform synchronous topic validation on random subset of the iHave messages
	if count > validationConfig.HardThreshold {
		err := c.validateTopics(validationConfig.ControlMsg, controlMessage, sampleSize)
		if err != nil {
			lg.Warn().
				Err(err).
				Bool(logging.KeySuspicious, true).
				Msg("topic validation pre-processing failed rejecting rpc control message")
			err = c.distributor.DistributeInvalidControlMessageNotification(p2p.NewInvalidControlMessageNotification(from, validationConfig.ControlMsg, count, err))
			if err != nil {
				lg.Error().
					Err(err).
					Bool(logging.KeySuspicious, true).
					Msg("failed to distribute invalid control message notification")
				return err
			}
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
	case !req.validationConfig.RateLimiter.Allow(req.Peer, int(count)): // check if Peer RPC messages are rate limited
		validationErr = NewRateLimitedControlMsgErr(req.validationConfig.ControlMsg)
	case count > req.validationConfig.SafetyThreshold && req.validationConfig.ControlMsg == p2p.CtrlMsgIHave:
		// we only perform async inspection on a sample size of iHave messages
		sampleSize := util.SampleN(len(req.ctrlMsg.GetIhave()), req.validationConfig.IHaveInspectionMaxSampleSize, req.validationConfig.IHaveAsyncInspectSampleSizePercentage)
		validationErr = c.validateTopics(req.validationConfig.ControlMsg, req.ctrlMsg, sampleSize)
	case count > req.validationConfig.SafetyThreshold:
		// check if Peer RPC messages Count greater than safety threshold further inspect each message individually
		validationErr = c.validateTopics(req.validationConfig.ControlMsg, req.ctrlMsg, 0)
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
	case p2p.CtrlMsgIHave:
		return uint64(len(ctrlMsg.GetIhave()))
	default:
		return 0
	}
}

// validateTopics ensures all topics in the specified control message are valid flow topic/channel and no duplicate topics exist.
// A sampleSize is only used when validating the topics of iHave control messages types because the number of iHave messages that
// can exist in a single RPC is unbounded.
// All errors returned from this function can be considered benign.
func (c *ControlMsgValidationInspector) validateTopics(ctrlMsgType p2p.ControlMessageType, ctrlMsg *pubsub_pb.ControlMessage, sampleSize uint) error {
	seen := make(map[channels.Topic]struct{})
	validateTopic := func(topic channels.Topic) error {
		if _, ok := seen[topic]; ok {
			return NewIDuplicateTopicErr(topic)
		}
		seen[topic] = struct{}{}
		err := c.validateTopic(topic)
		if err != nil {
			return NewInvalidTopicErr(topic, sampleSize, err)
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
	case p2p.CtrlMsgIHave:
		// for iHave control message topic validation we only validate a random subset of the messages
		iHaves := ctrlMsg.GetIhave()
		err := flowrand.Samples(uint(len(iHaves)), sampleSize, func(i, j uint) {
			iHaves[i], iHaves[j] = iHaves[j], iHaves[i]
		})
		if err != nil {
			return fmt.Errorf("failed to get random sample of ihave control messages: %w", err)
		}
		for i := uint(0); i < sampleSize; i++ {
			topic := channels.Topic(iHaves[i].GetTopicID())
			err = validateTopic(topic)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// validateTopic the topic is a valid flow topic/channel.
// All errors returned from this function can be considered benign.
func (c *ControlMsgValidationInspector) validateTopic(topic channels.Topic) error {
	err := channels.IsValidFlowTopic(topic, c.sporkID)
	if err != nil {
		return err
	}
	return nil
}
