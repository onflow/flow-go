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
	invalidCtrlMessageErrorCount           *prometheus.CounterVec
	rpcCtrlMsgTruncation                   prometheus.HistogramVec
	receivedIWantMsgCount                  prometheus.Counter
	receivedIWantMsgIDsHistogram           prometheus.Histogram
	receivedIHaveMsgCount                  prometheus.Counter
	receivedIHaveMsgIDsHistogram           prometheus.HistogramVec
	receivedPruneCount                     prometheus.Counter
	receivedGraftCount                     prometheus.Counter
	receivedPublishMessageCount            prometheus.Counter
	incomingRpcCount                       prometheus.Counter
}

var _ module.GossipSubRpcValidationInspectorMetrics = (*GossipSubRpcValidationInspectorMetrics)(nil)

// NewGossipSubRPCValidationInspectorMetrics returns a new *GossipSubRpcValidationInspectorMetrics.
func NewGossipSubRPCValidationInspectorMetrics(prefix string) *GossipSubRpcValidationInspectorMetrics {
	gc := &GossipSubRpcValidationInspectorMetrics{
		prefix: prefix,
	}
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

	gc.invalidCtrlMessageErrorCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gc.prefix + "rpc_invalid_control_message_error_total",
			Help:      "the number of invalid control message errors for a specific msg type",
		}, []string{LabelCtrlMsgType},
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
		Help:      "number of received iwant messages from gossipsub protocol",
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
		Help:      "number of received graft messages from gossipsub protocol",
	})

	gc.receivedPruneCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "gossipsub_received_prune_total",
		Help:      "number of received prune messages from gossipsub protocol",
	})

	gc.receivedPublishMessageCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "gossipsub_received_publish_message_total",
		Help:      "number of received publish messages from gossipsub protocol",
	})

	gc.incomingRpcCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceNetwork,
		Subsystem: subsystemGossip,
		Name:      gc.prefix + "gossipsub_incoming_rpc_total",
		Help:      "number of incoming rpc messages from gossipsub protocol",
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

// InvalidControlMessageNotificationError tracks the number of errors in each invalid control message notification over time per msg type.
func (c *GossipSubRpcValidationInspectorMetrics) InvalidControlMessageNotificationError(msgType p2pmsg.ControlMessageType, count float64) {
	c.invalidCtrlMessageErrorCount.WithLabelValues(msgType.String()).Add(count)
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
