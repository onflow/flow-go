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
	p2pconfig "github.com/onflow/flow-go/network/p2p/config"
	"github.com/onflow/flow-go/network/p2p/inspector/internal/cache"
	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/utils/logging"
	flowrand "github.com/onflow/flow-go/utils/rand"
)

const (
	RPCInspectionDisabledWarning     = "rpc inspection disabled for all control message types, skipping inspection"
	GraftInspectionDisabledWarning   = "rpc graft inspection disabled skipping"
	PruneInspectionDisabledWarning   = "rpc prune inspection disabled skipping"
	IWantInspectionDisabledWarning   = "rpc iwant inspection disabled skipping"
	IHaveInspectionDisabledWarning   = "rpc ihave inspection disabled skipping"
	PublishInspectionDisabledWarning = "rpc publish message inspection disabled skipping"

	RPCTruncationDisabledWarning            = "rpc truncation disabled for all control message types, skipping truncation"
	GraftTruncationDisabledWarning          = "rpc graft truncation disabled skipping"
	PruneTruncationDisabledWarning          = "rpc prune truncation disabled skipping"
	IHaveTruncationDisabledWarning          = "rpc ihave truncation disabled skipping"
	IHaveMessageIDTruncationDisabledWarning = "ihave message ids truncation disabled skipping"
	IWantTruncationDisabledWarning          = "rpc iwant truncation disabled skipping"
	IWantMessageIDTruncationDisabledWarning = "iwant message ids truncation disabled skipping"

	// rpcInspectorComponentName the rpc inspector component name.
	rpcInspectorComponentName = "gossipsub_rpc_validation_inspector"
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
	config *p2pconfig.RpcValidationInspector
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
	topicOracle func() p2p.TopicProvider
}

type InspectorParams struct {
	// Logger the logger used by the inspector.
	Logger zerolog.Logger `validate:"required"`
	// SporkID the current spork ID.
	SporkID flow.Identifier `validate:"required"`
	// Config inspector configuration.
	Config *p2pconfig.RpcValidationInspector `validate:"required"`
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
	// TopicOracle callback used to retrieve the current subscribed topics of the libp2p node.
	// It is set as a callback to avoid circular dependencies between the topic oracle and the inspector.
	TopicOracle func() p2p.TopicProvider `validate:"required"`
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

	clusterPrefixedTracker, err := cache.NewClusterPrefixedMessagesReceivedTracker(params.Logger,
		params.Config.ClusterPrefixedMessage.ControlMsgsReceivedCacheSize,
		clusterPrefixedCacheCollector,
		params.Config.ClusterPrefixedMessage.ControlMsgsReceivedCacheDecay)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster prefix topics received tracker")
	}

	if params.Config.PublishMessages.MaxSampleSize < params.Config.PublishMessages.ErrorThreshold {
		return nil, fmt.Errorf("rpc message max sample size must be greater than or equal to rpc message error threshold, got %d and %d respectively",
			params.Config.PublishMessages.MaxSampleSize,
			params.Config.PublishMessages.ErrorThreshold)
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
		topicOracle:    params.TopicOracle,
	}

	store := queue.NewHeroStore(params.Config.InspectionQueue.Size, params.Logger, inspectMsgQueueCacheCollector)

	pool := worker.NewWorkerPoolBuilder[*InspectRPCRequest](lg, store, c.processInspectRPCReq).Build()

	c.workerPool = pool

	builder := component.NewComponentManagerBuilder()
	builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		c.logger.Debug().Msg("starting rpc inspector distributor")
		c.ctx = ctx
		c.distributor.Start(ctx)
		select {
		case <-ctx.Done():
			c.logger.Debug().Msg("rpc inspector distributor startup aborted; context cancelled")
		case <-c.distributor.Ready():
			c.logger.Debug().Msg("rpc inspector distributor started")
			ready()
		}
		<-ctx.Done()
		c.logger.Debug().Msg("rpc inspector distributor stopped")
		<-c.distributor.Done()
		c.logger.Debug().Msg("rpc inspector distributor shutdown complete")
	})
	for i := 0; i < c.config.InspectionQueue.NumberOfWorkers; i++ {
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

// Inspect is called by gossipsub upon reception of a rpc from a remote  node.
// It creates a new InspectRPCRequest for the RPC to be inspected async by the worker pool.
// Args:
//   - from: the sender.
//   - rpc: the control message RPC.
//
// Returns:
//   - error: if a new inspect rpc request cannot be created, all errors returned are considered irrecoverable.
func (c *ControlMsgValidationInspector) Inspect(from peer.ID, rpc *pubsub.RPC) error {
	if c.config.InspectionProcess.Inspect.Disabled {
		c.logger.
			Trace().
			Str("peer_id", p2plogging.PeerId(from)).
			Bool(logging.KeyNetworkingSecurity, true).
			Msg(RPCInspectionDisabledWarning)
		return nil
	}

	// first truncate the rpc to the configured max sample size; if needed
	c.truncateRPC(from, rpc)

	// second, queue further async inspection
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
// Args:
//   - req: the inspect rpc request.
//
// Returns:
//   - error: no error is expected to be returned from this func as they are logged and distributed in invalid control message notifications.
func (c *ControlMsgValidationInspector) processInspectRPCReq(req *InspectRPCRequest) error {
	c.updateMetrics(req.Peer, req.rpc)
	c.metrics.AsyncProcessingStarted()
	start := time.Now()
	defer func() {
		c.metrics.AsyncProcessingFinished(time.Since(start))
	}()

	activeClusterIDS := c.tracker.GetActiveClusterIds()
	for _, ctrlMsgType := range p2pmsg.ControlMessageTypes() {
		switch ctrlMsgType {
		case p2pmsg.CtrlMsgGraft:
			err, topicType := c.inspectGraftMessages(req.Peer, req.rpc.GetControl().GetGraft(), activeClusterIDS)
			if err != nil {
				c.logAndDistributeAsyncInspectErrs(req, p2pmsg.CtrlMsgGraft, err, 1, topicType)
				return nil
			}
		case p2pmsg.CtrlMsgPrune:
			err, topicType := c.inspectPruneMessages(req.Peer, req.rpc.GetControl().GetPrune(), activeClusterIDS)
			if err != nil {
				c.logAndDistributeAsyncInspectErrs(req, p2pmsg.CtrlMsgPrune, err, 1, topicType)
				return nil
			}
		case p2pmsg.CtrlMsgIWant:
			err := c.inspectIWantMessages(req.Peer, req.rpc.GetControl().GetIwant())
			if err != nil {
				c.logAndDistributeAsyncInspectErrs(req, p2pmsg.CtrlMsgIWant, err, 1, p2p.CtrlMsgNonClusterTopicType)
				return nil
			}
		case p2pmsg.CtrlMsgIHave:
			err, topicType := c.inspectIHaveMessages(req.Peer, req.rpc.GetControl().GetIhave(), activeClusterIDS)
			if err != nil {
				c.logAndDistributeAsyncInspectErrs(req, p2pmsg.CtrlMsgIHave, err, 1, topicType)
				return nil
			}
		}
	}

	// inspect rpc publish messages after all control message validation has passed
	err, errCount := c.inspectRpcPublishMessages(req.Peer, req.rpc.GetPublish(), activeClusterIDS)
	if err != nil {
		c.logAndDistributeAsyncInspectErrs(req, p2pmsg.RpcPublishMessage, err, errCount, p2p.CtrlMsgNonClusterTopicType)
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
	} else if id.IsEjected() {
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
	if !c.config.InspectionProcess.Inspect.EnableGraft {
		c.logger.
			Trace().
			Str("peer_id", p2plogging.PeerId(from)).
			Bool(logging.KeyNetworkingSecurity, true).
			Msg(GraftInspectionDisabledWarning)
		return nil, p2p.CtrlMsgNonClusterTopicType
	}

	duplicateTopicTracker := make(duplicateStrTracker)
	totalDuplicateTopicIds := 0
	totalInvalidTopicIdErrs := 0
	defer func() {
		// regardless of inspection result, update metrics
		c.metrics.OnGraftMessageInspected(totalDuplicateTopicIds)
	}()

	for _, graft := range grafts {
		topic := channels.Topic(graft.GetTopicID())
		if duplicateTopicTracker.track(topic.String()) > 1 {
			// ideally, a GRAFT message should not have any duplicate topics, hence a topic ID is counted as a duplicate only if it is repeated more than once.
			totalDuplicateTopicIds++
			// check if the total number of duplicates exceeds the configured threshold.
			if totalDuplicateTopicIds > c.config.GraftPrune.DuplicateTopicIdThreshold {
				c.metrics.OnGraftDuplicateTopicIdsExceedThreshold()
				return NewDuplicateTopicErr(topic.String(), totalDuplicateTopicIds, p2pmsg.CtrlMsgGraft), p2p.CtrlMsgNonClusterTopicType
			}
		}
		err, ctrlMsgType := c.validateTopic(from, topic, activeClusterIDS)
		if err != nil {
			totalInvalidTopicIdErrs++
			// TODO: consider adding a threshold for this error similar to the duplicate topic id threshold.
			c.metrics.OnInvalidTopicIdDetectedForControlMessage(p2pmsg.CtrlMsgGraft)
			if totalInvalidTopicIdErrs > c.config.GraftPrune.InvalidTopicIdThreshold {
				return err, ctrlMsgType
			}
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
	if !c.config.InspectionProcess.Inspect.EnablePrune {
		c.logger.
			Trace().
			Str("peer_id", p2plogging.PeerId(from)).
			Bool(logging.KeyNetworkingSecurity, true).
			Msg(PruneInspectionDisabledWarning)
		return nil, p2p.CtrlMsgNonClusterTopicType
	}
	tracker := make(duplicateStrTracker)
	totalDuplicateTopicIds := 0
	totalInvalidTopicIdErrs := 0
	defer func() {
		// regardless of inspection result, update metrics
		c.metrics.OnPruneMessageInspected(totalDuplicateTopicIds)
	}()
	for _, prune := range prunes {
		topic := channels.Topic(prune.GetTopicID())
		if tracker.track(topic.String()) > 1 {
			// ideally, a PRUNE message should not have any duplicate topics, hence a topic ID is counted as a duplicate only if it is repeated more than once.
			totalDuplicateTopicIds++
			// check if the total number of duplicates exceeds the configured threshold.
			if totalDuplicateTopicIds > c.config.GraftPrune.DuplicateTopicIdThreshold {
				c.metrics.OnPruneDuplicateTopicIdsExceedThreshold()
				return NewDuplicateTopicErr(topic.String(), totalDuplicateTopicIds, p2pmsg.CtrlMsgPrune), p2p.CtrlMsgNonClusterTopicType
			}
		}
		err, ctrlMsgType := c.validateTopic(from, topic, activeClusterIDS)
		if err != nil {
			totalInvalidTopicIdErrs++
			// TODO: consider adding a threshold for this error similar to the duplicate topic id threshold.
			c.metrics.OnInvalidTopicIdDetectedForControlMessage(p2pmsg.CtrlMsgPrune)
			if totalInvalidTopicIdErrs > c.config.GraftPrune.InvalidTopicIdThreshold {
				return err, ctrlMsgType
			}
		}
	}
	return nil, p2p.CtrlMsgNonClusterTopicType
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
func (c *ControlMsgValidationInspector) inspectIHaveMessages(from peer.ID, ihaves []*pubsub_pb.ControlIHave, activeClusterIDS flow.ChainIDList) (error, p2p.CtrlMsgTopicType) {
	if !c.config.InspectionProcess.Inspect.EnableIHave {
		c.logger.
			Trace().
			Str("peer_id", p2plogging.PeerId(from)).
			Bool(logging.KeyNetworkingSecurity, true).
			Msg(IHaveInspectionDisabledWarning)
		return nil, p2p.CtrlMsgNonClusterTopicType
	}

	if len(ihaves) == 0 {
		return nil, p2p.CtrlMsgNonClusterTopicType
	}
	lg := c.logger.With().
		Str("peer_id", p2plogging.PeerId(from)).
		Int("sample_size", len(ihaves)).
		Int("max_sample_size", c.config.IHave.MessageCountThreshold).
		Logger()
	duplicateTopicTracker := make(duplicateStrTracker)
	duplicateMessageIDTracker := make(duplicateStrTracker)
	totalMessageIds := 0
	totalDuplicateTopicIds := 0
	totalDuplicateMessageIds := 0
	totalInvalidTopicIdErrs := 0
	defer func() {
		// regardless of inspection result, update metrics
		c.metrics.OnIHaveMessagesInspected(totalDuplicateTopicIds, totalDuplicateMessageIds)
	}()
	for _, ihave := range ihaves {
		messageIds := ihave.GetMessageIDs()
		topic := ihave.GetTopicID()
		totalMessageIds += len(messageIds)

		// first check if the topic is valid, fail fast if it is not
		err, ctrlMsgType := c.validateTopic(from, channels.Topic(topic), activeClusterIDS)
		if err != nil {
			totalInvalidTopicIdErrs++
			// TODO: consider adding a threshold for this error similar to the duplicate topic id threshold.
			c.metrics.OnInvalidTopicIdDetectedForControlMessage(p2pmsg.CtrlMsgIHave)
			if totalInvalidTopicIdErrs > c.config.IHave.InvalidTopicIdThreshold {
				return err, ctrlMsgType
			}
		}

		// then track the topic ensuring it is not beyond a duplicate threshold.
		if duplicateTopicTracker.track(topic) > 1 {
			totalDuplicateTopicIds++
			// the topic is duplicated, check if the total number of duplicates exceeds the configured threshold
			if totalDuplicateTopicIds > c.config.IHave.DuplicateTopicIdThreshold {
				c.metrics.OnIHaveDuplicateTopicIdsExceedThreshold()
				return NewDuplicateTopicErr(topic, totalDuplicateTopicIds, p2pmsg.CtrlMsgIHave), p2p.CtrlMsgNonClusterTopicType
			}
		}

		for _, messageID := range messageIds {
			if duplicateMessageIDTracker.track(messageID) > 1 {
				totalDuplicateMessageIds++
				// the message is duplicated, check if the total number of duplicates exceeds the configured threshold
				if totalDuplicateMessageIds > c.config.IHave.DuplicateMessageIdThreshold {
					c.metrics.OnIHaveDuplicateMessageIdsExceedThreshold()
					return NewDuplicateMessageIDErr(messageID, totalDuplicateMessageIds, p2pmsg.CtrlMsgIHave), p2p.CtrlMsgNonClusterTopicType
				}
			}
		}
	}
	lg.Debug().
		Int("total_message_ids", totalMessageIds).
		Int("total_duplicate_topic_ids", totalDuplicateTopicIds).
		Int("total_duplicate_message_ids", totalDuplicateMessageIds).
		Msg("ihave control message validation complete")
	return nil, p2p.CtrlMsgNonClusterTopicType
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
	if !c.config.InspectionProcess.Inspect.EnableIWant {
		c.logger.
			Trace().
			Str("peer_id", p2plogging.PeerId(from)).
			Bool(logging.KeyNetworkingSecurity, true).
			Msg(IWantInspectionDisabledWarning)
		return nil
	}

	if len(iWants) == 0 {
		return nil
	}
	lastHighest := c.rpcTracker.LastHighestIHaveRPCSize()
	lg := c.logger.With().
		Str("peer_id", p2plogging.PeerId(from)).
		Uint("max_sample_size", c.config.IWant.MessageCountThreshold).
		Int64("last_highest_ihave_rpc_size", lastHighest).
		Logger()
	duplicateMsgIdTracker := make(duplicateStrTracker)
	cacheMisses := 0
	duplicateMessageIds := 0
	defer func() {
		// regardless of inspection result, update metrics
		c.metrics.OnIWantMessagesInspected(duplicateMessageIds, cacheMisses)
	}()

	lg = lg.With().
		Int("iwant_msg_count", len(iWants)).
		Int("cache_misses_threshold", c.config.IWant.CacheMissThreshold).
		Int("duplicates_threshold", c.config.IWant.DuplicateMsgIdThreshold).Logger()

	lg.Trace().Msg("validating sample of message ids from iwant control message")

	totalMessageIds := 0
	for _, iWant := range iWants {
		messageIds := iWant.GetMessageIDs()
		messageIDCount := uint(len(messageIds))
		for _, messageID := range messageIds {
			// check duplicate allowed threshold
			if duplicateMsgIdTracker.track(messageID) > 1 {
				// ideally, an iWant message should not have any duplicate message IDs, hence a message id is considered duplicate when it is repeated more than once.
				duplicateMessageIds++
				if duplicateMessageIds > c.config.IWant.DuplicateMsgIdThreshold {
					c.metrics.OnIWantDuplicateMessageIdsExceedThreshold()
					return NewIWantDuplicateMsgIDThresholdErr(duplicateMessageIds, messageIDCount, c.config.IWant.DuplicateMsgIdThreshold)
				}
			}
			// check cache miss threshold
			if !c.rpcTracker.WasIHaveRPCSent(messageID) {
				cacheMisses++
				if cacheMisses > c.config.IWant.CacheMissThreshold {
					c.metrics.OnIWantCacheMissMessageIdsExceedThreshold()
					return NewIWantCacheMissThresholdErr(cacheMisses, messageIDCount, c.config.IWant.CacheMissThreshold)
				}
			}
			duplicateMsgIdTracker.track(messageID)
			totalMessageIds++
		}
	}

	lg.Debug().
		Int("total_message_ids", totalMessageIds).
		Int("cache_misses", cacheMisses).
		Int("total_duplicate_message_ids", duplicateMessageIds).
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
	if !c.config.InspectionProcess.Inspect.EnablePublish {
		c.logger.
			Trace().
			Str("peer_id", p2plogging.PeerId(from)).
			Bool(logging.KeyNetworkingSecurity, true).
			Msg(PublishInspectionDisabledWarning)
		return nil, 0
	}

	totalMessages := len(messages)
	if totalMessages == 0 {
		return nil, 0
	}
	sampleSize := c.config.PublishMessages.MaxSampleSize
	if sampleSize > totalMessages {
		sampleSize = totalMessages
	}
	c.performSample(p2pmsg.RpcPublishMessage, uint(totalMessages), uint(sampleSize), func(i, j uint) {
		messages[i], messages[j] = messages[j], messages[i]
	})

	subscribedTopics := c.topicOracle().GetTopics()
	hasSubscription := func(topic string) bool {
		for _, subscribedTopic := range subscribedTopics {
			if topic == subscribedTopic {
				return true
			}
		}
		return false
	}
	var errs *multierror.Error
	invalidTopicIdsCount := 0
	invalidSubscriptionsCount := 0
	invalidSendersCount := 0
	defer func() {
		// regardless of inspection result, update metrics
		errCnt := 0
		if errs != nil {
			errCnt = errs.Len()
		}
		c.metrics.OnPublishMessageInspected(errCnt, invalidTopicIdsCount, invalidSubscriptionsCount, invalidSendersCount)
	}()
	for _, message := range messages[:sampleSize] {
		if c.networkingType == network.PrivateNetwork {
			err := c.checkPubsubMessageSender(message)
			if err != nil {
				invalidSendersCount++
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
			invalidTopicIdsCount++
			errs = multierror.Append(errs, err)
			continue
		}

		if !hasSubscription(topic.String()) {
			invalidSubscriptionsCount++
			errs = multierror.Append(errs, fmt.Errorf("subscription for topic %s not found", topic))
		}
	}

	// return an error when we exceed the error threshold
	if errs != nil && errs.Len() > c.config.PublishMessages.ErrorThreshold {
		c.metrics.OnPublishMessagesInspectionErrorExceedsThreshold()
		return NewInvalidRpcPublishMessagesErr(errs.ErrorOrNil(), errs.Len()), uint64(errs.Len())
	}

	return nil, 0
}

// truncateRPC truncates the RPC by truncating each control message type using the configured max sample size values.
// Args:
// - from: peer ID of the sender.
// - rpc: the pubsub RPC.
func (c *ControlMsgValidationInspector) truncateRPC(from peer.ID, rpc *pubsub.RPC) {
	if c.config.InspectionProcess.Truncate.Disabled {
		c.logger.
			Trace().
			Str("peer_id", p2plogging.PeerId(from)).
			Bool(logging.KeyNetworkingSecurity, true).
			Msg(RPCTruncationDisabledWarning)
		return
	}

	for _, ctlMsgType := range p2pmsg.ControlMessageTypes() {
		switch ctlMsgType {
		case p2pmsg.CtrlMsgGraft:
			c.truncateGraftMessages(from, rpc)
		case p2pmsg.CtrlMsgPrune:
			c.truncatePruneMessages(from, rpc)
		case p2pmsg.CtrlMsgIHave:
			c.truncateIHaveMessages(from, rpc)
			c.truncateIHaveMessageIds(from, rpc)
		case p2pmsg.CtrlMsgIWant:
			c.truncateIWantMessages(from, rpc)
			c.truncateIWantMessageIds(from, rpc)
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
func (c *ControlMsgValidationInspector) truncateGraftMessages(from peer.ID, rpc *pubsub.RPC) {
	if !c.config.InspectionProcess.Truncate.EnableGraft {
		c.logger.
			Trace().
			Str("peer_id", p2plogging.PeerId(from)).
			Bool(logging.KeyNetworkingSecurity, true).
			Msg(GraftTruncationDisabledWarning)
		return
	}

	grafts := rpc.GetControl().GetGraft()
	originalGraftSize := len(grafts)
	if originalGraftSize <= c.config.GraftPrune.MessageCountThreshold {
		return // nothing to truncate
	}

	// truncate grafts and update metrics
	sampleSize := c.config.GraftPrune.MessageCountThreshold
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
func (c *ControlMsgValidationInspector) truncatePruneMessages(from peer.ID, rpc *pubsub.RPC) {
	if !c.config.InspectionProcess.Truncate.EnablePrune {
		c.logger.
			Trace().
			Str("peer_id", p2plogging.PeerId(from)).
			Bool(logging.KeyNetworkingSecurity, true).
			Msg(PruneTruncationDisabledWarning)
		return
	}

	prunes := rpc.GetControl().GetPrune()
	originalPruneSize := len(prunes)
	if originalPruneSize <= c.config.GraftPrune.MessageCountThreshold {
		return // nothing to truncate
	}

	sampleSize := c.config.GraftPrune.MessageCountThreshold
	c.performSample(p2pmsg.CtrlMsgPrune, uint(originalPruneSize), uint(sampleSize), func(i, j uint) {
		prunes[i], prunes[j] = prunes[j], prunes[i]
	})
	rpc.Control.Prune = prunes[:sampleSize]
	c.metrics.OnControlMessagesTruncated(p2pmsg.CtrlMsgPrune, originalPruneSize-len(rpc.Control.Prune))
}

// truncateIHaveMessages truncates the iHaves control messages in the RPC. If the total number of iHaves in the RPC exceeds the configured
// MessageCountThreshold the list of iHaves will be truncated.
// Args:
//   - rpc: the rpc message to truncate.
func (c *ControlMsgValidationInspector) truncateIHaveMessages(from peer.ID, rpc *pubsub.RPC) {
	if !c.config.InspectionProcess.Truncate.EnableIHave {
		c.logger.
			Trace().
			Str("peer_id", p2plogging.PeerId(from)).
			Bool(logging.KeyNetworkingSecurity, true).
			Msg(IHaveTruncationDisabledWarning)
		return
	}

	ihaves := rpc.GetControl().GetIhave()
	originalIHaveCount := len(ihaves)
	if originalIHaveCount == 0 {
		return
	}

	if originalIHaveCount > c.config.IHave.MessageCountThreshold {
		// truncate ihaves and update metrics
		sampleSize := c.config.IHave.MessageCountThreshold
		if sampleSize > originalIHaveCount {
			sampleSize = originalIHaveCount
		}
		c.performSample(p2pmsg.CtrlMsgIHave, uint(originalIHaveCount), uint(sampleSize), func(i, j uint) {
			ihaves[i], ihaves[j] = ihaves[j], ihaves[i]
		})
		rpc.Control.Ihave = ihaves[:sampleSize]
		c.metrics.OnControlMessagesTruncated(p2pmsg.CtrlMsgIHave, originalIHaveCount-len(rpc.Control.Ihave))
	}
}

// truncateIHaveMessageIds truncates the message ids for each iHave control message in the RPC. If the total number of message ids in a single iHave exceeds the configured
// MessageIdCountThreshold the list of message ids will be truncated. Before message ids are truncated the iHave control messages should have been truncated themselves.
// Args:
//   - rpc: the rpc message to truncate.
func (c *ControlMsgValidationInspector) truncateIHaveMessageIds(from peer.ID, rpc *pubsub.RPC) {
	if !c.config.InspectionProcess.Truncate.EnableIHaveMessageIds {
		c.logger.
			Trace().
			Str("peer_id", p2plogging.PeerId(from)).
			Bool(logging.KeyNetworkingSecurity, true).
			Msg(IHaveMessageIDTruncationDisabledWarning)
		return
	}

	for _, ihave := range rpc.GetControl().GetIhave() {
		messageIDs := ihave.GetMessageIDs()
		originalMessageIdCount := len(messageIDs)
		if originalMessageIdCount == 0 {
			continue // nothing to truncate; skip
		}

		if originalMessageIdCount > c.config.IHave.MessageIdCountThreshold {
			sampleSize := c.config.IHave.MessageIdCountThreshold
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
// MessageCountThreshold the list of iWants will be truncated.
// Args:
//   - rpc: the rpc message to truncate.
func (c *ControlMsgValidationInspector) truncateIWantMessages(from peer.ID, rpc *pubsub.RPC) {
	if !c.config.InspectionProcess.Truncate.EnableIWant {
		c.logger.
			Trace().
			Str("peer_id", p2plogging.PeerId(from)).
			Bool(logging.KeyNetworkingSecurity, true).
			Msg(IWantTruncationDisabledWarning)
		return
	}

	iWants := rpc.GetControl().GetIwant()
	originalIWantCount := uint(len(iWants))
	if originalIWantCount == 0 {
		return
	}

	if originalIWantCount > c.config.IWant.MessageCountThreshold {
		// truncate iWants and update metrics
		sampleSize := c.config.IWant.MessageCountThreshold
		if sampleSize > originalIWantCount {
			sampleSize = originalIWantCount
		}
		c.performSample(p2pmsg.CtrlMsgIWant, originalIWantCount, sampleSize, func(i, j uint) {
			iWants[i], iWants[j] = iWants[j], iWants[i]
		})
		rpc.Control.Iwant = iWants[:sampleSize]
		c.metrics.OnControlMessagesTruncated(p2pmsg.CtrlMsgIWant, int(originalIWantCount)-len(rpc.Control.Iwant))
	}
}

// truncateIWantMessageIds truncates the message ids for each iWant control message in the RPC. If the total number of message ids in a single iWant exceeds the configured
// MessageIdCountThreshold the list of message ids will be truncated. Before message ids are truncated the iWant control messages should have been truncated themselves.
// Args:
//   - rpc: the rpc message to truncate.
func (c *ControlMsgValidationInspector) truncateIWantMessageIds(from peer.ID, rpc *pubsub.RPC) {
	if !c.config.InspectionProcess.Truncate.EnableIWantMessageIds {
		c.logger.
			Trace().
			Str("peer_id", p2plogging.PeerId(from)).
			Bool(logging.KeyNetworkingSecurity, true).
			Msg(IWantMessageIDTruncationDisabledWarning)
		return
	}

	lastHighest := c.rpcTracker.LastHighestIHaveRPCSize()
	lg := c.logger.With().
		Str("peer_id", p2plogging.PeerId(from)).
		Uint("max_sample_size", c.config.IWant.MessageCountThreshold).
		Int64("last_highest_ihave_rpc_size", lastHighest).
		Logger()

	sampleSize := int(10 * lastHighest)
	if sampleSize == 0 || sampleSize > c.config.IWant.MessageIdCountThreshold {
		// invalid or 0 sample size is suspicious
		lg.Warn().Str(logging.KeySuspicious, "true").Msg("zero or invalid sample size, using default max sample size")
		sampleSize = c.config.IWant.MessageIdCountThreshold
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
func (c *ControlMsgValidationInspector) validateTopic(from peer.ID, topic channels.Topic, activeClusterIds flow.ChainIDList) (error, p2p.CtrlMsgTopicType) {
	channel, ok := channels.ChannelFromTopic(topic)
	if !ok {
		return channels.NewInvalidTopicErr(topic, fmt.Errorf("failed to get channel from topic")), p2p.CtrlMsgNonClusterTopicType
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
	return nil, p2p.CtrlMsgNonClusterTopicType
}

// validateClusterPrefixedTopic validates cluster prefixed topics.
// Expected error returns during normal operations:
//   - ErrActiveClusterIdsNotSet: if the cluster ID provider is not set.
//   - channels.InvalidTopicErr: if topic is invalid.
//   - channels.UnknownClusterIDErr: if the topic contains a cluster ID prefix that is not in the active cluster IDs list.
//
// In the case where an ErrActiveClusterIdsNotSet or UnknownClusterIDErr is encountered and the cluster prefixed topic received
// tracker for the peer is less than or equal to the configured HardThreshold an error will only be logged and not returned.
// At the point where the hard threshold is crossed the error will be returned and the sender will start to be penalized.
// Any errors encountered while incrementing or loading the cluster prefixed control message gauge for a peer will result in an irrecoverable error being thrown, these
// errors are unexpected and irrecoverable indicating a bug.
func (c *ControlMsgValidationInspector) validateClusterPrefixedTopic(from peer.ID, topic channels.Topic, activeClusterIds flow.ChainIDList) error {
	lg := c.logger.With().
		Str("from", p2plogging.PeerId(from)).
		Logger()

	// only staked nodes are expected to participate on cluster prefixed topics
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
		if ok := c.checkClusterPrefixHardThreshold(nodeID); ok {
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
// the configured HardThreshold, false otherwise.
// If any error is encountered while loading from the tracker this func will throw an error on the signaler context, these errors
// are unexpected and irrecoverable indicating a bug.
func (c *ControlMsgValidationInspector) checkClusterPrefixHardThreshold(nodeID flow.Identifier) bool {
	gauge, err := c.tracker.Load(nodeID)
	if err != nil {
		// irrecoverable error encountered
		c.logAndThrowError(fmt.Errorf("cluster prefixed control message gauge during hard threshold check failed for node %s: %w", nodeID, err))
	}
	return gauge <= c.config.ClusterPrefixedMessage.HardThreshold
}

// logAndDistributeErr logs the provided error and attempts to disseminate an invalid control message validation notification for the error.
// Args:
//   - req: inspect rpc request that failed validation.
//   - ctlMsgType: the control message type of the rpc message that caused the error.
//   - err: the error that occurred.
//   - count: the number of occurrences of the error.
//   - isClusterPrefixed: indicates if the errors occurred on a cluster prefixed topic.
func (c *ControlMsgValidationInspector) logAndDistributeAsyncInspectErrs(req *InspectRPCRequest, ctlMsgType p2pmsg.ControlMessageType, err error, count uint64, topicType p2p.CtrlMsgTopicType) {
	lg := c.logger.With().
		Err(err).
		Str("control_message_type", ctlMsgType.String()).
		Bool(logging.KeySuspicious, true).
		Bool(logging.KeyNetworkingSecurity, true).
		Str("topic_type", topicType.String()).
		Uint64("error_count", count).
		Str("peer_id", p2plogging.PeerId(req.Peer)).
		Logger()

	switch {
	case IsErrActiveClusterIDsNotSet(err):
		c.metrics.OnActiveClusterIDsNotSetErr()
		lg.Warn().Msg("active cluster ids not set")
	case IsErrUnstakedPeer(err):
		c.metrics.OnUnstakedPeerInspectionFailed()
		lg.Warn().Msg("control message received from unstaked peer")
	default:
		distErr := c.distributor.Distribute(p2p.NewInvalidControlMessageNotification(req.Peer, ctlMsgType, err, count, topicType))
		if distErr != nil {
			lg.Error().
				Err(distErr).
				Msg("failed to distribute invalid control message notification")
			return
		}
		lg.Error().Msg("rpc control message async inspection failed")
		c.metrics.OnInvalidControlMessageNotificationSent()
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
