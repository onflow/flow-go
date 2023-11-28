package validation

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-multierror"
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
	"github.com/onflow/flow-go/network"
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
	tracker    *cache.ClusterPrefixedMessagesReceivedTracker
	idProvider module.IdentityProvider
	rpcTracker p2p.RpcControlTracking
	// networkingType indicates public or private network, rpc publish messages are inspected for unstaked senders when running the private network.
	networkingType network.NetworkingType
	// topicOracle callback used to retrieve the current subscribed topics of the libp2p node.
	topicOracle func() []string
}

type InspectorParams struct {
	// Logger the logger used by the inspector.
	Logger zerolog.Logger `validate:"required"`
	// SporkID the current spork ID.
	SporkID flow.Identifier `validate:"required"`
	// Config inspector configuration.
	Config *p2pconf.GossipSubRPCValidationInspectorConfigs `validate:"required"`
	// Distributor gossipsub inspector notification distributor.
	Distributor p2p.GossipSubInspectorNotifDistributor `validate:"required"`
	// HeroCacheMetricsFactory the metrics factory.
	HeroCacheMetricsFactory metrics.HeroCacheMetricsFactory `validate:"required"`
	// IdProvider identity provider is used to get the flow identifier for a peer.
	IdProvider module.IdentityProvider `validate:"required"`
	// InspectorMetrics metrics for the validation inspector.
	InspectorMetrics module.GossipSubRpcValidationInspectorMetrics `validate:"required"`
	// RpcTracker tracker used to track iHave RPC's sent and last size.
	RpcTracker p2p.RpcControlTracking `validate:"required"`
	// NetworkingType the networking type of the node.
	NetworkingType network.NetworkingType `validate:"required"`
}

var _ component.Component = (*ControlMsgValidationInspector)(nil)
var _ p2p.GossipSubMsgValidationRpcInspector = (*ControlMsgValidationInspector)(nil)
var _ protocol.Consumer = (*ControlMsgValidationInspector)(nil)

// NewControlMsgValidationInspector returns new ControlMsgValidationInspector
// Args:
//   - *InspectorParams: params used to create the inspector.
//
// Returns:
//   - *ControlMsgValidationInspector: a new control message validation inspector.
//   - error: an error if there is any error while creating the inspector. All errors are irrecoverable and unexpected.
func NewControlMsgValidationInspector(params *InspectorParams) (*ControlMsgValidationInspector, error) {
	err := validator.New().Struct(params)
	if err != nil {
		return nil, fmt.Errorf("inspector params validation failed: %w", err)
	}
	lg := params.Logger.With().Str("component", "gossip_sub_rpc_validation_inspector").Logger()

	inspectMsgQueueCacheCollector := metrics.GossipSubRPCInspectorQueueMetricFactory(params.HeroCacheMetricsFactory, params.NetworkingType)
	clusterPrefixedCacheCollector := metrics.GossipSubRPCInspectorClusterPrefixedCacheMetricFactory(params.HeroCacheMetricsFactory, params.NetworkingType)

	clusterPrefixedTracker, err := cache.NewClusterPrefixedMessagesReceivedTracker(params.Logger, params.Config.ClusterPrefixedControlMsgsReceivedCacheSize, clusterPrefixedCacheCollector, params.Config.ClusterPrefixedControlMsgsReceivedCacheDecay)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster prefix topics received tracker")
	}

	if params.Config.RpcMessageMaxSampleSize < params.Config.RpcMessageErrorThreshold {
		return nil, fmt.Errorf("rpc message max sample size must be greater than or equal to rpc message error threshold, got %d and %d respectively", params.Config.RpcMessageMaxSampleSize, params.Config.RpcMessageErrorThreshold)
	}

	c := &ControlMsgValidationInspector{
		logger:         lg,
		sporkID:        params.SporkID,
		config:         params.Config,
		distributor:    params.Distributor,
		tracker:        clusterPrefixedTracker,
		rpcTracker:     params.RpcTracker,
		idProvider:     params.IdProvider,
		metrics:        params.InspectorMetrics,
		networkingType: params.NetworkingType,
	}

	store := queue.NewHeroStore(params.Config.CacheSize, params.Logger, inspectMsgQueueCacheCollector)

	pool := worker.NewWorkerPoolBuilder[*InspectRPCRequest](lg, store, c.processInspectRPCReq).Build()

	c.workerPool = pool

	builder := component.NewComponentManagerBuilder()
	builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		c.ctx = ctx
		c.distributor.Start(ctx)
		select {
		case <-ctx.Done():
		case <-c.distributor.Ready():
			ready()
		}
		<-c.distributor.Done()
	})
	for i := 0; i < c.config.NumberOfWorkers; i++ {
		builder.AddWorker(pool.WorkerLogic())
	}
	c.Component = builder.Build()
	return c, nil
}

func (c *ControlMsgValidationInspector) Start(parent irrecoverable.SignalerContext) {
	if c.topicOracle == nil {
		parent.Throw(fmt.Errorf("control message validation inspector topic oracle not set"))
	}
	c.Component.Start(parent)
}

// Name returns the name of the rpc inspector.
func (c *ControlMsgValidationInspector) Name() string {
	return rpcInspectorComponentName
}

// ActiveClustersChanged consumes cluster ID update protocol events.
func (c *ControlMsgValidationInspector) ActiveClustersChanged(clusterIDList flow.ChainIDList) {
	c.tracker.StoreActiveClusterIds(clusterIDList)
}

// SetTopicOracle Sets the topic oracle. The topic oracle is used to determine the list of topics that the node is subscribed to.
// If an oracle is not set, the node will not be able to determine the list of topics that the node is subscribed to.
// This func is expected to be called once and will return an error on all subsequent calls.
// All errors returned from this func are considered irrecoverable.
func (c *ControlMsgValidationInspector) SetTopicOracle(topicOracle func() []string) error {
	if c.topicOracle != nil {
		return fmt.Errorf("topic oracle already set")
	}
	c.topicOracle = topicOracle
	return nil
}

// Inspect is called by gossipsub upon reception of a rpc from a remote  node.
// It creates a new InspectRPCRequest for the RPC to be inspected async by the worker pool.
// Args:
//   - from: the sender.
//   - rpc: the control message RPC.
//
// Returns:
//   - error: if a new inspect rpc request cannot be created, all errors returned are considered irrecoverable.
func (c *ControlMsgValidationInspector) Inspect(from peer.ID, rpc *pubsub.RPC) error {
	c.truncateRPC(from, rpc)
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

// processInspectRPCReq func used by component workers to perform further inspection of RPC control messages that will validate ensure all control message
// types are valid in the RPC.
// Args:
//   - req: the inspect rpc request.
//
// Returns:
//   - error: no error is expected to be returned from this func as they are logged and distributed in invalid control message notifications.
func (c *ControlMsgValidationInspector) processInspectRPCReq(req *InspectRPCRequest) error {
	c.metrics.AsyncProcessingStarted()
	start := time.Now()
	defer func() {
		c.metrics.AsyncProcessingFinished(time.Since(start))
	}()

	activeClusterIDS := c.tracker.GetActiveClusterIds()
	for _, ctrlMsgType := range p2pmsg.ControlMessageTypes() {
		switch ctrlMsgType {
		case p2pmsg.CtrlMsgGraft:
			err, isClusterPrefixed := c.inspectGraftMessages(req.Peer, req.rpc.GetControl().GetGraft(), activeClusterIDS)
			if err != nil {
				c.logAndDistributeAsyncInspectErrs(req, p2pmsg.CtrlMsgGraft, err, 1, isClusterPrefixed)
				return nil
			}
		case p2pmsg.CtrlMsgPrune:
			err, isClusterPrefixed := c.inspectPruneMessages(req.Peer, req.rpc.GetControl().GetPrune(), activeClusterIDS)
			if err != nil {
				c.logAndDistributeAsyncInspectErrs(req, p2pmsg.CtrlMsgPrune, err, 1, isClusterPrefixed)
				return nil
			}
		case p2pmsg.CtrlMsgIWant:
			err := c.inspectIWantMessages(req.Peer, req.rpc.GetControl().GetIwant())
			if err != nil {
				c.logAndDistributeAsyncInspectErrs(req, p2pmsg.CtrlMsgIWant, err, 1, false)
				return nil
			}
		case p2pmsg.CtrlMsgIHave:
			err, isClusterPrefixed := c.inspectIHaveMessages(req.Peer, req.rpc.GetControl().GetIhave(), activeClusterIDS)
			if err != nil {
				c.logAndDistributeAsyncInspectErrs(req, p2pmsg.CtrlMsgIHave, err, 1, isClusterPrefixed)
				return nil
			}
		}
	}

	// inspect rpc publish messages after all control message validation has passed
	err, errCount := c.inspectRpcPublishMessages(req.Peer, req.rpc.GetPublish(), activeClusterIDS)
	if err != nil {
		c.logAndDistributeAsyncInspectErrs(req, p2pmsg.RpcPublishMessage, err, errCount, false)
		return nil
	}

	return nil
}

// checkPubsubMessageSender checks the sender of the sender of pubsub message to ensure they are not unstaked, or ejected.
// This check is only required on private networks.
// Args:
//   - message: the pubsub message.
//
// Returns:
//   - error: if the peer ID cannot be created from bytes, sender is unknown or the identity is ejected.
//
// All errors returned from this function can be considered benign.
func (c *ControlMsgValidationInspector) checkPubsubMessageSender(message *pubsub_pb.Message) error {
	pid, err := peer.IDFromBytes(message.GetFrom())
	if err != nil {
		return fmt.Errorf("failed to get peer ID from bytes: %w", err)
	}
	if id, ok := c.idProvider.ByPeerID(pid); !ok {
		return fmt.Errorf("received rpc publish message from unstaked peer: %s", pid)
	} else if id.Ejected {
		return fmt.Errorf("received rpc publish message from ejected peer: %s", pid)
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
// - bool: true if an error is returned and the topic that failed validation was a cluster prefixed topic, false otherwise.
func (c *ControlMsgValidationInspector) inspectGraftMessages(from peer.ID, grafts []*pubsub_pb.ControlGraft, activeClusterIDS flow.ChainIDList) (error, p2p.CtrlMsgTopicType) {
	tracker := make(duplicateStrTracker)
	for _, graft := range grafts {
		topic := channels.Topic(graft.GetTopicID())
		if tracker.isDuplicate(topic.String()) {
			return NewDuplicateTopicErr(topic.String(), p2pmsg.CtrlMsgGraft), p2p.CtrlMsgNonClusterTopicType
		}
		tracker.set(topic.String())
		err, ctrlMsgType := c.validateTopic(from, topic, activeClusterIDS)
		if err != nil {
			return err, ctrlMsgType
		}
	}
	return nil, p2p.CtrlMsgNonClusterTopicType
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
//   - bool: true if an error is returned and the topic that failed validation was a cluster prefixed topic, false otherwise.
func (c *ControlMsgValidationInspector) inspectPruneMessages(from peer.ID, prunes []*pubsub_pb.ControlPrune, activeClusterIDS flow.ChainIDList) (error, p2p.CtrlMsgTopicType) {
	tracker := make(duplicateStrTracker)
	for _, prune := range prunes {
		topic := channels.Topic(prune.GetTopicID())
		if tracker.isDuplicate(topic.String()) {
			return NewDuplicateTopicErr(topic.String(), p2pmsg.CtrlMsgPrune), false
		}
		tracker.set(topic.String())
		err, ctrlMsgType := c.validateTopic(from, topic, activeClusterIDS)
		if err != nil {
			return err, isClusterPrefixed
		}
	}
	return nil, false
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
//   - bool: true if an error is returned and the topic that failed validation was a cluster prefixed topic, false otherwise.
func (c *ControlMsgValidationInspector) inspectIHaveMessages(from peer.ID, ihaves []*pubsub_pb.ControlIHave, activeClusterIDS flow.ChainIDList) (error, bool) {
	if len(ihaves) == 0 {
		return nil, false
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
			return NewDuplicateTopicErr(topic, p2pmsg.CtrlMsgIHave), false
		}
		duplicateTopicTracker.set(topic)
		err, isClusterPrefixed := c.validateTopic(from, channels.Topic(topic), activeClusterIDS)
		if err != nil {
			return err, isClusterPrefixed
		}

		for _, messageID := range messageIds {
			if duplicateMessageIDTracker.isDuplicate(messageID) {
				return NewDuplicateTopicErr(messageID, p2pmsg.CtrlMsgIHave), false
			}
			duplicateMessageIDTracker.set(messageID)
		}
	}
	lg.Debug().
		Int("total_message_ids", totalMessageIds).
		Msg("ihave control message validation complete")
	return nil, false
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
	checkCacheMisses := len(iWants) >= c.config.IWantRPCInspectionConfig.CacheMissCheckSize
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

// inspectRpcPublishMessages inspects a sample of the RPC gossip messages and performs topic validation that ensures the following:
// - Topics are known flow topics.
// - Topics are valid flow topics.
// - Topics are in the nodes subscribe topics list.
// If more than half the topics in the sample contain invalid topics an error will be returned.
// Args:
// - from: peer ID of the sender.
// - messages: rpc publish messages.
// - activeClusterIDS: the list of active cluster ids.
// Returns:
// - InvalidRpcPublishMessagesErr: if the amount of invalid messages exceeds the configured RPCMessageErrorThreshold.
// - int: the number of invalid pubsub messages
func (c *ControlMsgValidationInspector) inspectRpcPublishMessages(from peer.ID, messages []*pubsub_pb.Message, activeClusterIDS flow.ChainIDList) (error, uint64) {
	totalMessages := len(messages)
	if totalMessages == 0 {
		return nil, 0
	}
	sampleSize := c.config.RpcMessageMaxSampleSize
	if sampleSize > totalMessages {
		sampleSize = totalMessages
	}
	c.performSample(p2pmsg.RpcPublishMessage, uint(totalMessages), uint(sampleSize), func(i, j uint) {
		messages[i], messages[j] = messages[j], messages[i]
	})

	subscribedTopics := c.topicOracle()
	hasSubscription := func(topic string) bool {
		for _, subscribedTopic := range subscribedTopics {
			if topic == subscribedTopic {
				return true
			}
		}
		return false
	}
	var errs *multierror.Error
	for _, message := range messages[:sampleSize] {
		if c.networkingType == network.PrivateNetwork {
			err := c.checkPubsubMessageSender(message)
			if err != nil {
				errs = multierror.Append(errs, err)
				continue
			}
		}
		topic := channels.Topic(message.GetTopic())
		// The boolean value returned when validating a topic, indicating whether the topic is cluster-prefixed or not, is intentionally ignored.
		// This is because we have already set a threshold for errors allowed on publish messages. Reducing the penalty further based on
		// cluster prefix status is unnecessary when the error threshold is exceeded.
		err, _ := c.validateTopic(from, topic, activeClusterIDS)
		if err != nil {
			// we can skip checking for subscription of topic that failed validation and continue
			errs = multierror.Append(errs, err)
			continue
		}

		if !hasSubscription(topic.String()) {
			errs = multierror.Append(errs, fmt.Errorf("subscription for topic %s not found", topic))
		}
	}

	// return an error when we exceed the error threshold
	if errs != nil && errs.Len() > c.config.RpcMessageErrorThreshold {
		return NewInvalidRpcPublishMessagesErr(errs.ErrorOrNil(), errs.Len()), uint64(errs.Len())
	}

	return nil, 0
}

// truncateRPC truncates the RPC by truncating each control message type using the configured max sample size values.
// Args:
// - from: peer ID of the sender.
// - rpc: the pubsub RPC.
func (c *ControlMsgValidationInspector) truncateRPC(from peer.ID, rpc *pubsub.RPC) {
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
}

// truncateGraftMessages truncates the Graft control messages in the RPC. If the total number of Grafts in the RPC exceeds the configured
// GraftPruneMessageMaxSampleSize the list of Grafts will be truncated.
// Args:
//   - rpc: the rpc message to truncate.
func (c *ControlMsgValidationInspector) truncateGraftMessages(rpc *pubsub.RPC) {
	grafts := rpc.GetControl().GetGraft()
	totalGrafts := len(grafts)
	if totalGrafts == 0 {
		return
	}
	sampleSize := c.config.GraftPruneMessageMaxSampleSize
	if sampleSize > totalGrafts {
		sampleSize = totalGrafts
	}
	c.performSample(p2pmsg.CtrlMsgGraft, uint(totalGrafts), uint(sampleSize), func(i, j uint) {
		grafts[i], grafts[j] = grafts[j], grafts[i]
	})
	rpc.Control.Graft = grafts[:sampleSize]
}

// truncatePruneMessages truncates the Prune control messages in the RPC. If the total number of Prunes in the RPC exceeds the configured
// GraftPruneMessageMaxSampleSize the list of Prunes will be truncated.
// Args:
//   - rpc: the rpc message to truncate.
func (c *ControlMsgValidationInspector) truncatePruneMessages(rpc *pubsub.RPC) {
	prunes := rpc.GetControl().GetPrune()
	totalPrunes := len(prunes)
	if totalPrunes == 0 {
		return
	}
	sampleSize := c.config.GraftPruneMessageMaxSampleSize
	if sampleSize > totalPrunes {
		sampleSize = totalPrunes
	}
	c.performSample(p2pmsg.CtrlMsgPrune, uint(totalPrunes), uint(sampleSize), func(i, j uint) {
		prunes[i], prunes[j] = prunes[j], prunes[i]
	})
	rpc.Control.Prune = prunes[:sampleSize]
}

// truncateIHaveMessages truncates the iHaves control messages in the RPC. If the total number of iHaves in the RPC exceeds the configured
// MaxSampleSize the list of iHaves will be truncated.
// Args:
//   - rpc: the rpc message to truncate.
func (c *ControlMsgValidationInspector) truncateIHaveMessages(rpc *pubsub.RPC) {
	ihaves := rpc.GetControl().GetIhave()
	totalIHaves := len(ihaves)
	if totalIHaves == 0 {
		return
	}
	sampleSize := c.config.IHaveRPCInspectionConfig.MaxSampleSize
	if sampleSize > totalIHaves {
		sampleSize = totalIHaves
	}

	c.performSample(p2pmsg.CtrlMsgIHave, uint(totalIHaves), uint(sampleSize), func(i, j uint) {
		ihaves[i], ihaves[j] = ihaves[j], ihaves[i]
	})
	rpc.Control.Ihave = ihaves[:sampleSize]
	c.truncateIHaveMessageIds(rpc)
}

// truncateIHaveMessageIds truncates the message ids for each iHave control message in the RPC. If the total number of message ids in a single iHave exceeds the configured
// MaxMessageIDSampleSize the list of message ids will be truncated. Before message ids are truncated the iHave control messages should have been truncated themselves.
// Args:
//   - rpc: the rpc message to truncate.
func (c *ControlMsgValidationInspector) truncateIHaveMessageIds(rpc *pubsub.RPC) {
	for _, ihave := range rpc.GetControl().GetIhave() {
		messageIDs := ihave.GetMessageIDs()
		totalMessageIDs := len(messageIDs)
		if totalMessageIDs == 0 {
			return
		}
		sampleSize := c.config.IHaveRPCInspectionConfig.MaxMessageIDSampleSize
		if sampleSize > totalMessageIDs {
			sampleSize = totalMessageIDs
		}
		c.performSample(p2pmsg.CtrlMsgIHave, uint(totalMessageIDs), uint(sampleSize), func(i, j uint) {
			messageIDs[i], messageIDs[j] = messageIDs[j], messageIDs[i]
		})
		ihave.MessageIDs = messageIDs[:sampleSize]
	}
}

// truncateIWantMessages truncates the iWant control messages in the RPC. If the total number of iWants in the RPC exceeds the configured
// MaxSampleSize the list of iWants will be truncated.
// Args:
//   - rpc: the rpc message to truncate.
func (c *ControlMsgValidationInspector) truncateIWantMessages(from peer.ID, rpc *pubsub.RPC) {
	iWants := rpc.GetControl().GetIwant()
	totalIWants := uint(len(iWants))
	if totalIWants == 0 {
		return
	}
	sampleSize := c.config.IWantRPCInspectionConfig.MaxSampleSize
	if sampleSize > totalIWants {
		sampleSize = totalIWants
	}
	c.performSample(p2pmsg.CtrlMsgIWant, totalIWants, sampleSize, func(i, j uint) {
		iWants[i], iWants[j] = iWants[j], iWants[i]
	})
	rpc.Control.Iwant = iWants[:sampleSize]
	c.truncateIWantMessageIds(from, rpc)
}

// truncateIWantMessageIds truncates the message ids for each iWant control message in the RPC. If the total number of message ids in a single iWant exceeds the configured
// MaxMessageIDSampleSize the list of message ids will be truncated. Before message ids are truncated the iWant control messages should have been truncated themselves.
// Args:
//   - rpc: the rpc message to truncate.
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
		totalMessageIDs := len(messageIDs)
		if totalMessageIDs == 0 {
			return
		}
		if sampleSize > totalMessageIDs {
			sampleSize = totalMessageIDs
		}
		c.performSample(p2pmsg.CtrlMsgIWant, uint(totalMessageIDs), uint(sampleSize), func(i, j uint) {
			messageIDs[i], messageIDs[j] = messageIDs[j], messageIDs[i]
		})
		iWant.MessageIDs = messageIDs[:sampleSize]
	}
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
func (c *ControlMsgValidationInspector) validateTopic(from peer.ID, topic channels.Topic, activeClusterIds flow.ChainIDList) (error, bool) {
	channel, ok := channels.ChannelFromTopic(topic)
	if !ok {
		return channels.NewInvalidTopicErr(topic, fmt.Errorf("failed to get channel from topic")), false
	}

	// handle cluster prefixed topics
	if channels.IsClusterChannel(channel) {
		return c.validateClusterPrefixedTopic(from, topic, activeClusterIds), p2p.CtrlMsgTopicTypeClusterPrefixed
	}

	// non cluster prefixed topic validation
	err := channels.IsValidNonClusterFlowTopic(topic, c.sporkID)
	if err != nil {
		return err, p2p.CtrlMsgNonClusterTopicType
	}
	return nil, false
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
// Args:
//   - req: inspect rpc request that failed validation.
//   - ctlMsgType: the control message type of the rpc message that caused the error.
//   - err: the error that occurred.
//   - count: the number of occurrences of the error.
//   - isClusterPrefixed: indicates if the errors occurred on a cluster prefixed topic.
func (c *ControlMsgValidationInspector) logAndDistributeAsyncInspectErrs(req *InspectRPCRequest, ctlMsgType p2pmsg.ControlMessageType, err error, count uint64, isClusterPrefixed bool) {
	lg := c.logger.With().
		Err(err).
		Str("control_message_type", ctlMsgType.String()).
		Bool(logging.KeySuspicious, true).
		Bool(logging.KeyNetworkingSecurity, true).
		Bool("is_cluster_prefixed", isClusterPrefixed).
		Uint64("error_count", count).
		Str("peer_id", p2plogging.PeerId(req.Peer)).
		Logger()

	switch {
	case IsErrActiveClusterIDsNotSet(err):
		lg.Warn().Msg("active cluster ids not set")
	case IsErrUnstakedPeer(err):
		lg.Warn().Msg("control message received from unstaked peer")
	default:
		distErr := c.distributor.Distribute(p2p.NewInvalidControlMessageNotification(req.Peer, ctlMsgType, err, count, isClusterPrefixed))
		if distErr != nil {
			lg.Error().
				Err(distErr).
				Msg("failed to distribute invalid control message notification")
		}
		lg.Error().Msg("rpc control message async inspection failed")
	}
}

// logAndThrowError logs and throws irrecoverable errors on the context.
// Args:
//
//	err: the error encountered.
func (c *ControlMsgValidationInspector) logAndThrowError(err error) {
	c.logger.Error().
		Err(err).
		Bool(logging.KeySuspicious, true).
		Bool(logging.KeyNetworkingSecurity, true).
		Msg("unexpected irrecoverable error encountered")
	c.ctx.Throw(err)
}
