package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

const (
	labelIHaveMessageIds = "ihave_message_ids"
	labelIWantMessageIds = "iwant_message_ids"
)

// GossipSubRpcValidationInspectorMetrics metrics collector for the gossipsub RPC validation inspector.
type GossipSubRpcValidationInspectorMetrics struct {
	prefix                                 string
	rpcCtrlMsgInAsyncPreProcessingGauge    prometheus.Gauge
	rpcCtrlMsgAsyncProcessingTimeHistogram prometheus.Histogram
	rpcCtrlMsgTruncation                   prometheus.HistogramVec
	ctrlMsgInvalidTopicIdCount             prometheus.CounterVec
	receivedIWantMsgCount                  prometheus.Counter
	receivedIWantMsgIDsHistogram           prometheus.Histogram
	receivedIHaveMsgCount                  prometheus.Counter
	receivedIHaveMsgIDsHistogram           prometheus.HistogramVec
	receivedPruneCount                     prometheus.Counter
	receivedGraftCount                     prometheus.Counter
	receivedPublishMessageCount            prometheus.Counter
	incomingRpcCount                       prometheus.Counter

	// graft inspection
	graftDuplicateTopicIdsHistogram            prometheus.Histogram
	graftInvalidTopicIdsHistogram              prometheus.Histogram
	graftDuplicateTopicIdsExceedThresholdCount prometheus.Counter
	graftInvalidTopicIdsExceedThresholdCount   prometheus.Counter

	// prune inspection
	pruneDuplicateTopicIdsHistogram            prometheus.Histogram
	pruneInvalidTopicIdsHistogram              prometheus.Histogram
	pruneDuplicateTopicIdsExceedThresholdCount prometheus.Counter
	pruneInvalidTopicIdsExceedThresholdCount   prometheus.Counter

	// iHave inspection
	iHaveDuplicateMessageIdHistogram            prometheus.Histogram
	iHaveDuplicateTopicIdHistogram              prometheus.Histogram
	iHaveInvalidTopicIdHistogram                prometheus.Histogram
	iHaveDuplicateMessageIdExceedThresholdCount prometheus.Counter
	iHaveDuplicateTopicIdExceedThresholdCount   prometheus.Counter
	iHaveInvalidTopicIdExceedThresholdCount     prometheus.Counter

	// iWant inspection
	iWantDuplicateMessageIdHistogram            prometheus.Histogram
	iWantCacheMissHistogram                     prometheus.Histogram
	iWantDuplicateMessageIdExceedThresholdCount prometheus.Counter
	iWantCacheMissMessageIdExceedThresholdCount prometheus.Counter

	// inspection result
	errActiveClusterIdsNotSetCount             prometheus.Counter
	errUnstakedPeerInspectionFailedCount       prometheus.Counter
	invalidControlMessageNotificationSentCount prometheus.Counter
	unstakedNodeRPCRejectedCount               prometheus.Counter

	// publish messages
	publishMessageInspectionErrExceedThresholdCount prometheus.Counter
	publishMessageInvalidSenderCountHistogram       prometheus.Histogram
	publishMessageInvalidSubscriptionsHistogram     prometheus.Histogram
	publishMessageInvalidTopicIdHistogram           prometheus.Histogram
	publishMessageInspectedErrHistogram             prometheus.Histogram
}

var _ module.GossipSubRpcValidationInspectorMetrics = (*GossipSubRpcValidationInspectorMetrics)(nil)

// NewGossipSubRPCValidationInspectorMetrics returns a new *GossipSubRpcValidationInspectorMetrics.
func NewGossipSubRPCValidationInspectorMetrics(prefix string) *GossipSubRpcValidationInspectorMetrics {
	gc := &GossipSubRpcValidationInspectorMetrics{prefix: prefix}
	gc.rpcCtrlMsgInAsyncPreProcessingGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gc.prefix + "control_messages_in_async_processing_total",
			Help:      "the number of rpc control messages currently being processed asynchronously by workers from the rpc validator worker pool",
		},
	)
	gc.rpcCtrlMsgAsyncProcessingTimeHistogram = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gc.prefix + "rpc_control_message_validator_async_processing_time_seconds",
			Help:      "duration [seconds; measured with float64 precision] of how long it takes rpc control message validator to asynchronously process a rpc message",
			Buckets:   []float64{.1, 1},
		},
	)

	gc.rpcCtrlMsgTruncation = *promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_control_message_truncation",
		Help:      "the number of times a control message was truncated",
		Buckets:   []float64{10, 100, 1000},
	}, []string{LabelMessage})

	gc.receivedIHaveMsgCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "gossipsub_received_ihave_total",
		Help:      "number of received ihave messages from gossipsub protocol",
	})

	gc.receivedIHaveMsgIDsHistogram = *promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "gossipsub_received_ihave_message_ids",
		Help:      "histogram of received ihave message ids from gossipsub protocol per channel",
	}, []string{LabelChannel})

	gc.receivedIWantMsgCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "gossipsub_received_iwant_total",
		Help:      "total number of received iwant messages from gossipsub protocol",
	})

	gc.receivedIWantMsgIDsHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "gossipsub_received_iwant_message_ids",
		Help:      "histogram of received iwant message ids from gossipsub protocol per channel",
	})

	gc.receivedGraftCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "gossipsub_received_graft_total",
		Help:      "total number of received graft messages from gossipsub protocol",
	})

	gc.receivedPruneCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "gossipsub_received_prune_total",
		Help:      "total number of received prune messages from gossipsub protocol",
	})

	gc.receivedPublishMessageCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "gossipsub_received_publish_message_total",
		Help:      "total number of received publish messages from gossipsub protocol",
	})

	gc.incomingRpcCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "gossipsub_incoming_rpc_total",
		Help:      "total number of incoming rpc messages from gossipsub protocol",
	})

	gc.iHaveDuplicateMessageIdHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Buckets:   []float64{1, 100, 1000},
		Name:      gc.prefix + "rpc_inspection_ihave_duplicate_message_ids_count",
		Help:      "number of duplicate message ids received from gossipsub protocol during the async inspection of a single RPC",
	})

	gc.iHaveDuplicateTopicIdHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Buckets:   []float64{1, 100, 1000},
		Name:      gc.prefix + "rpc_inspection_ihave_duplicate_topic_ids_count",
		Help:      "number of duplicate topic ids received from gossipsub protocol during the async inspection of a single RPC",
	})

	gc.iHaveInvalidTopicIdHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Buckets:   []float64{1, 100, 1000},
		Name:      gc.prefix + "rpc_inspection_ihave_invalid_topic_ids_count",
		Help:      "number of invalid topic ids received from gossipsub protocol during the async inspection of a single RPC",
	})

	gc.iHaveDuplicateMessageIdExceedThresholdCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_ihave_duplicate_message_ids_exceed_threshold_total",
		Help:      "total number of times that the async inspection of iHave messages failed due to the number of duplicate message ids exceeding the threshold",
	})

	gc.iHaveDuplicateTopicIdExceedThresholdCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_ihave_duplicate_topic_ids_exceed_threshold_total",
		Help:      "total number of times that the async inspection of iHave messages failed due to the number of duplicate topic ids exceeding the threshold",
	})

	gc.iHaveInvalidTopicIdExceedThresholdCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_ihave_invalid_topic_ids_exceed_threshold_total",
		Help:      "total number of times that the async inspection of iHave messages failed due to the number of invalid topic ids exceeding the threshold",
	})

	gc.iWantDuplicateMessageIdHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_iwant_duplicate_message_ids_count",
		Buckets:   []float64{1, 100, 1000},
		Help:      "number of duplicate message ids received from gossipsub protocol during the async inspection of a single RPC",
	})

	gc.iWantCacheMissHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_iwant_cache_miss_message_ids_count",
		Buckets:   []float64{1, 100, 1000},
		Help:      "total number of cache miss message ids received from gossipsub protocol during the async inspection of a single RPC",
	})

	gc.iWantDuplicateMessageIdExceedThresholdCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_iwant_duplicate_message_ids_exceed_threshold_total",
		Help:      "total number of times that the async inspection of iWant messages failed due to the number of duplicate message ids ",
	})

	gc.iWantCacheMissMessageIdExceedThresholdCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_iwant_cache_miss_message_ids_exceed_threshold_total",
		Help:      "total number of times that the async inspection of iWant messages failed due to the number of cache miss message ids ",
	})

	gc.ctrlMsgInvalidTopicIdCount = *promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "control_message_invalid_topic_id_total",
		Help:      "total number of control messages with invalid topic id received from gossipsub protocol during the async inspection",
	}, []string{LabelMessage})

	gc.errActiveClusterIdsNotSetCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "active_cluster_ids_not_inspection_error_total",
		Help:      "total number of inspection errors due to active cluster ids not set inspection failure",
	})

	gc.errUnstakedPeerInspectionFailedCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "unstaked_peer_inspection_error_total",
		Help:      "total number of inspection errors due to unstaked peer inspection failure",
	})

	gc.invalidControlMessageNotificationSentCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "invalid_control_message_notification_sent_total",
		Help:      "number of invalid control message notifications (i.e., misbehavior report) sent due to async inspection of rpcs failure",
	})

	gc.unstakedNodeRPCRejectedCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "unstaked_node_rejection_total",
		Help:      "number of rpcs rejected from unstaked node",
	})

	gc.graftDuplicateTopicIdsHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_graft_duplicate_topic_ids_count",
		Buckets:   []float64{1, 100, 1000},
		Help:      "number of duplicate topic ids on graft messages of a single RPC during the async inspection, regardless of the result of the inspection",
	})

	gc.graftInvalidTopicIdsHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_graft_invalid_topic_ids_count",
		Buckets:   []float64{1, 100, 1000},
		Help:      "number of invalid topic ids on graft messages of a single RPC during the async inspection, regardless of the result of the inspection",
	})

	gc.graftDuplicateTopicIdsExceedThresholdCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_graft_duplicate_topic_ids_exceed_threshold_total",
		Help:      "number of times that the async inspection of graft messages of an rpc failed due to the number of duplicate topic ids exceeding the threshold",
	})

	gc.graftInvalidTopicIdsExceedThresholdCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_graft_invalid_topic_ids_exceed_threshold_total",
		Help:      "number of times that the async inspection of graft messages of an rpc failed due to the number of invalid topic ids exceeding the threshold",
	})

	gc.pruneDuplicateTopicIdsHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Buckets:   []float64{1, 100, 1000},
		Name:      gc.prefix + "rpc_inspection_prune_duplicate_topic_ids_count",
		Help:      "number of duplicate topic ids on prune messages of a single RPC during the async inspection, regardless of the result of the inspection",
	})

	gc.pruneInvalidTopicIdsHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Buckets:   []float64{1, 100, 1000},
		Name:      gc.prefix + "rpc_inspection_prune_invalid_topic_ids_count",
		Help:      "number of invalid topic ids on prune messages of a single RPC during the async inspection, regardless of the result of the inspection",
	})

	gc.pruneDuplicateTopicIdsExceedThresholdCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_prune_duplicate_topic_ids_exceed_threshold_total",
		Help:      "number of times that the async inspection of prune messages failed due to the number of duplicate topic ids exceeding the threshold",
	})

	gc.pruneInvalidTopicIdsExceedThresholdCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_prune_invalid_topic_ids_exceed_threshold_total",
		Help:      "number of times that the async inspection of prune messages failed due to the number of invalid topic ids exceeding the threshold",
	})

	gc.publishMessageInspectedErrHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "publish_message_inspected_error_count",
		Buckets:   []float64{10, 100, 1000},
		Help:      "number of errors that occurred during the async inspection of publish messages on a single RPC, regardless pof the result",
	})

	gc.publishMessageInvalidSenderCountHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_publish_message_invalid_sender_count",
		Buckets:   []float64{1, 100, 1000},
		Help:      "number of invalid senders observed during the async inspection of publish messages on a single RPC, regardless of the result",
	})

	gc.publishMessageInvalidTopicIdHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_publish_message_invalid_topic_id_count",
		Buckets:   []float64{1, 100, 1000},
		Help:      "number of invalid topic ids observed during the async inspection of publish messages on a single RPC, regardless of the result",
	})

	gc.publishMessageInvalidSubscriptionsHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "rpc_inspection_publish_message_invalid_subscriptions_count",
		Buckets:   []float64{1, 100, 1000},
		Help:      "number of invalid subscriptions observed during the async inspection of publish messages on a single RPC, regardless of the result",
	})

	gc.publishMessageInspectionErrExceedThresholdCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "publish_message_inspection_err_exceed_threshold_total",
		Help:      "number of rpcs fail on inspection due to published message inspection errors exceeding the threshold",
	})

	return gc
}

// AsyncProcessingStarted increments the metric tracking the number of RPC's being processed asynchronously by the rpc validation inspector.
func (c *GossipSubRpcValidationInspectorMetrics) AsyncProcessingStarted() {
	c.rpcCtrlMsgInAsyncPreProcessingGauge.Inc()
}

// AsyncProcessingFinished tracks the time spent by the rpc validation inspector to process an RPC asynchronously and decrements the metric tracking
// the number of RPC's being processed asynchronously by the rpc validation inspector.
func (c *GossipSubRpcValidationInspectorMetrics) AsyncProcessingFinished(duration time.Duration) {
	c.rpcCtrlMsgInAsyncPreProcessingGauge.Dec()
	c.rpcCtrlMsgAsyncProcessingTimeHistogram.Observe(duration.Seconds())
}

// OnControlMessagesTruncated tracks the number of times a control message was truncated.
// Args:
//
//	messageType: the type of the control message that was truncated
//	diff: the number of message ids truncated.
func (c *GossipSubRpcValidationInspectorMetrics) OnControlMessagesTruncated(messageType p2pmsg.ControlMessageType, diff int) {
	c.rpcCtrlMsgTruncation.WithLabelValues(messageType.String()).Observe(float64(diff))
}

// OnIHaveControlMessageIdsTruncated tracks the number of times message ids on an iHave message were truncated.
// Note that this function is called only when the message ids are truncated from an iHave message, not when the iHave message itself is truncated.
// This is different from the OnControlMessagesTruncated function which is called when a slice of control messages truncated from an RPC with all their message ids.
// Args:
//
//	diff: the number of actual messages truncated.
func (c *GossipSubRpcValidationInspectorMetrics) OnIHaveControlMessageIdsTruncated(diff int) {
	c.OnControlMessagesTruncated(labelIHaveMessageIds, diff)
}

// OnIWantControlMessageIdsTruncated tracks the number of times message ids on an iWant message were truncated.
// Note that this function is called only when the message ids are truncated from an iWant message, not when the iWant message itself is truncated.
// This is different from the OnControlMessagesTruncated function which is called when a slice of control messages truncated from an RPC with all their message ids.
// Args:
//
//	diff: the number of actual messages truncated.
func (c *GossipSubRpcValidationInspectorMetrics) OnIWantControlMessageIdsTruncated(diff int) {
	c.OnControlMessagesTruncated(labelIWantMessageIds, diff)
}

// OnIWantMessageIDsReceived tracks the number of message ids received by the node from other nodes on an RPC.
// Note: this function is called on each IWANT message received by the node.
// Args:
// - msgIdCount: the number of message ids received on the IWANT message.
func (c *GossipSubRpcValidationInspectorMetrics) OnIWantMessageIDsReceived(msgIdCount int) {
	c.receivedIWantMsgIDsHistogram.Observe(float64(msgIdCount))
}

// OnIHaveMessageIDsReceived tracks the number of message ids received by the node from other nodes on an iHave message.
// This function is called on each iHave message received by the node.
// Args:
// - channel: the channel on which the iHave message was received.
// - msgIdCount: the number of message ids received on the iHave message.
func (c *GossipSubRpcValidationInspectorMetrics) OnIHaveMessageIDsReceived(channel string, msgIdCount int) {
	c.receivedIHaveMsgIDsHistogram.WithLabelValues(channel).Observe(float64(msgIdCount))
}

// OnIncomingRpcReceived tracks the number of incoming RPC messages received by the node.
func (c *GossipSubRpcValidationInspectorMetrics) OnIncomingRpcReceived(iHaveCount, iWantCount, graftCount, pruneCount, msgCount int) {
	c.incomingRpcCount.Inc()
	c.receivedPublishMessageCount.Add(float64(msgCount))
	c.receivedPruneCount.Add(float64(pruneCount))
	c.receivedGraftCount.Add(float64(graftCount))
	c.receivedIWantMsgCount.Add(float64(iWantCount))
	c.receivedIHaveMsgCount.Add(float64(iHaveCount))
}

// OnIWantMessagesInspected tracks the number of duplicate and cache miss message ids received by the node on iWant messages at the end of the async inspection iWants
// across one RPC, regardless of the result of the inspection.
//
//	duplicateCount: the total number of duplicate message ids received by the node on the iWant messages at the end of the async inspection of the RPC.
//	cacheMissCount: the total number of cache miss message ids received by the node on the iWant message at the end of the async inspection of the RPC.
func (c *GossipSubRpcValidationInspectorMetrics) OnIWantMessagesInspected(duplicateCount int, cacheMissCount int) {
	c.iWantDuplicateMessageIdHistogram.Observe(float64(duplicateCount))
	c.iWantCacheMissHistogram.Observe(float64(cacheMissCount))
}

// OnIWantDuplicateMessageIdsExceedThreshold tracks the number of times that async inspection of iWant messages failed due to the total number of duplicate message ids
// received by the node on the iWant messages of a single RPC exceeding the threshold, which results in a misbehaviour report.
func (c *GossipSubRpcValidationInspectorMetrics) OnIWantDuplicateMessageIdsExceedThreshold() {
	c.iWantDuplicateMessageIdExceedThresholdCount.Inc()
}

// OnIWantCacheMissMessageIdsExceedThreshold tracks the number of times that async inspection of iWant messages failed due to the total
// number of cache miss message ids received by the node on the iWant messages of a single RPC exceeding the threshold, which results in a misbehaviour report.
func (c *GossipSubRpcValidationInspectorMetrics) OnIWantCacheMissMessageIdsExceedThreshold() {
	c.iWantCacheMissMessageIdExceedThresholdCount.Inc()
}

// OnIHaveMessagesInspected is called at the end of the async inspection of iHave messages of a single RPC, regardless of the result of the inspection.
// It tracks the number of duplicate topic ids and duplicate message ids received by the node on the iHave messages of that single RPC at the end of the async inspection iHaves.
// Args:
//
//	duplicateTopicIds: the total number of duplicate topic ids received by the node on the iHave messages at the end of the async inspection of the RPC.
//	duplicateMessageIds: the number of duplicate message ids received by the node on the iHave messages at the end of the async inspection of the RPC.
//	invalidTopicIds: the number of invalid message ids received by the node on the iHave messages at the end of the async inspection of the RPC.
func (c *GossipSubRpcValidationInspectorMetrics) OnIHaveMessagesInspected(duplicateTopicIds, duplicateMessageIds, invalidTopicIds int) {
	c.iHaveDuplicateTopicIdHistogram.Observe(float64(duplicateTopicIds))
	c.iHaveDuplicateMessageIdHistogram.Observe(float64(duplicateMessageIds))
	c.iHaveInvalidTopicIdHistogram.Observe(float64(invalidTopicIds))
}

// OnIHaveDuplicateTopicIdsExceedThreshold tracks the number of times that the async inspection of iHave messages of a single RPC failed due to the total number of duplicate topic ids
// received by the node on the iHave messages of that RPC exceeding the threshold, which results in a misbehaviour report.
func (c *GossipSubRpcValidationInspectorMetrics) OnIHaveDuplicateTopicIdsExceedThreshold() {
	c.iHaveDuplicateTopicIdExceedThresholdCount.Inc()
}

// OnIHaveDuplicateMessageIdsExceedThreshold tracks the number of times that the async inspection of iHave messages of a single RPC failed due to the total number of duplicate message ids
// received by the node on an iHave message exceeding the threshold, which results in a misbehaviour report.
func (c *GossipSubRpcValidationInspectorMetrics) OnIHaveDuplicateMessageIdsExceedThreshold() {
	c.iHaveDuplicateMessageIdExceedThresholdCount.Inc()
}

// OnIHaveInvalidTopicIdsExceedThreshold tracks the number of times that the async inspection of iHave messages of a single RPC failed due to the total number of invalid topic ids
// received by the node on the iHave messages of that RPC exceeding the threshold, which results in a misbehaviour report.
func (c *GossipSubRpcValidationInspectorMetrics) OnIHaveInvalidTopicIdsExceedThreshold() {
	c.iHaveInvalidTopicIdExceedThresholdCount.Inc()
}

// OnInvalidTopicIdDetectedForControlMessage tracks the number of times that the async inspection of a control message type on a single RPC failed due to an invalid topic id.
// Args:
// - messageType: the type of the control message that was truncated.
func (c *GossipSubRpcValidationInspectorMetrics) OnInvalidTopicIdDetectedForControlMessage(messageType p2pmsg.ControlMessageType) {
	c.ctrlMsgInvalidTopicIdCount.WithLabelValues(messageType.String()).Inc()
}

// OnActiveClusterIDsNotSetErr tracks the number of times that the async inspection of a control message type on a single RPC failed due to active cluster ids not set inspection failure.
// This is not causing a misbehaviour report.
func (c *GossipSubRpcValidationInspectorMetrics) OnActiveClusterIDsNotSetErr() {
	c.errActiveClusterIdsNotSetCount.Inc()
}

// OnUnstakedPeerInspectionFailed tracks the number of times that the async inspection of a control message type on a single RPC failed due to unstaked peer inspection failure.
// This is not causing a misbehaviour report.
func (c *GossipSubRpcValidationInspectorMetrics) OnUnstakedPeerInspectionFailed() {
	c.errUnstakedPeerInspectionFailedCount.Inc()
}

// OnInvalidControlMessageNotificationSent tracks the number of times that the async inspection of a control message failed and resulted in dissemination of an invalid control message was sent (i.e., a
// misbehavior report).
func (c *GossipSubRpcValidationInspectorMetrics) OnInvalidControlMessageNotificationSent() {
	c.invalidControlMessageNotificationSentCount.Inc()
}

// OnRpcRejectedFromUnknownSender tracks the number of rpc's rejected from unstaked nodes.
func (c *GossipSubRpcValidationInspectorMetrics) OnRpcRejectedFromUnknownSender() {
	c.unstakedNodeRPCRejectedCount.Inc()
}

// OnPruneDuplicateTopicIdsExceedThreshold tracks the number of times that the async inspection of prune messages for an RPC failed due to the number of duplicate topic ids
// received by the node on prune messages of the same RPC excesses threshold, which results in a misbehaviour report.
func (c *GossipSubRpcValidationInspectorMetrics) OnPruneDuplicateTopicIdsExceedThreshold() {
	c.pruneDuplicateTopicIdsExceedThresholdCount.Inc()
}

// OnPruneInvalidTopicIdsExceedThreshold tracks the number of times that the async inspection of prune messages for an RPC failed due to the number of invalid topic ids
// received by the node on prune messages of the same RPC excesses threshold, which results in a misbehaviour report.
func (c *GossipSubRpcValidationInspectorMetrics) OnPruneInvalidTopicIdsExceedThreshold() {
	c.pruneInvalidTopicIdsExceedThresholdCount.Inc()
}

// OnPruneMessageInspected is called at the end of the async inspection of prune messages of the RPC, regardless of the result of the inspection.
// Args:
//
//	duplicateTopicIds: the number of duplicate topic ids received by the node on the prune messages of the RPC at the end of the async inspection prunes.
//	invalidTopicIds: the number of invalid message ids received by the node on the prune messages at the end of the async inspection of the RPC.
func (c *GossipSubRpcValidationInspectorMetrics) OnPruneMessageInspected(duplicateTopicIds, invalidTopicIds int) {
	c.pruneDuplicateTopicIdsHistogram.Observe(float64(duplicateTopicIds))
	c.pruneInvalidTopicIdsHistogram.Observe(float64(invalidTopicIds))
}

// OnGraftDuplicateTopicIdsExceedThreshold tracks the number of times that the async inspection of a graft message failed due to the number of duplicate topic ids.
// received by the node on graft messages of the same rpc excesses threshold, which results in a misbehaviour report.
func (c *GossipSubRpcValidationInspectorMetrics) OnGraftDuplicateTopicIdsExceedThreshold() {
	c.graftDuplicateTopicIdsExceedThresholdCount.Inc()
}

// OnGraftInvalidTopicIdsExceedThreshold tracks the number of times that the async inspection of the graft messages of a single RPC failed due to the number of invalid topic ids
// received by the node on graft messages of the same RPC excesses threshold, which results in a misbehaviour report.
func (c *GossipSubRpcValidationInspectorMetrics) OnGraftInvalidTopicIdsExceedThreshold() {
	c.graftInvalidTopicIdsExceedThresholdCount.Inc()
}

// OnGraftMessageInspected is called at the end of the async inspection of graft messages of a single RPC, regardless of the result of the inspection.
// Args:
//
//	duplicateTopicIds: the number of duplicate topic ids received by the node on the graft messages at the end of the async inspection of a single RPC.
//	invalidTopicIds: the number of invalid message ids received by the node on the graft messages at the end of the async inspection of the RPC.
func (c *GossipSubRpcValidationInspectorMetrics) OnGraftMessageInspected(duplicateTopicIds, invalidTopicIds int) {
	c.graftDuplicateTopicIdsHistogram.Observe(float64(duplicateTopicIds))
	c.graftInvalidTopicIdsHistogram.Observe(float64(invalidTopicIds))
}

// OnPublishMessageInspected is called at the end of the async inspection of publish messages of a single RPC, regardless of the result of the inspection.
// It tracks the total number of errors detected during the async inspection of the rpc together with their individual breakdown.
// Args:
// - errCount: the number of errors that occurred during the async inspection of publish messages.
// - invalidTopicIdsCount: the number of times that an invalid topic id was detected during the async inspection of publish messages.
// - invalidSubscriptionsCount: the number of times that an invalid subscription was detected during the async inspection of publish messages.
// - invalidSendersCount: the number of times that an invalid sender was detected during the async inspection of publish messages.
func (c *GossipSubRpcValidationInspectorMetrics) OnPublishMessageInspected(totalErrCount int, invalidTopicIdsCount int, invalidSubscriptionsCount int, invalidSendersCount int) {
	c.publishMessageInspectedErrHistogram.Observe(float64(totalErrCount))
	c.publishMessageInvalidSenderCountHistogram.Observe(float64(invalidSendersCount))
	c.publishMessageInvalidSubscriptionsHistogram.Observe(float64(invalidSubscriptionsCount))
	c.publishMessageInvalidTopicIdHistogram.Observe(float64(invalidTopicIdsCount))
}

// OnPublishMessagesInspectionErrorExceedsThreshold tracks the number of times that async inspection of publish messages failed due to the number of errors exceeding the threshold.
// Note that it causes a misbehaviour report.
func (c *GossipSubRpcValidationInspectorMetrics) OnPublishMessagesInspectionErrorExceedsThreshold() {
	c.publishMessageInspectionErrExceedThresholdCount.Inc()
}
