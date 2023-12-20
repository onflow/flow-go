package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

const (
	labelIHaveMessage = "ihave_actual_message"
)

// GossipSubRpcValidationInspectorMetrics metrics collector for the gossipsub RPC validation inspector.
type GossipSubRpcValidationInspectorMetrics struct {
	prefix                                 string
	rpcCtrlMsgInAsyncPreProcessingGauge    prometheus.Gauge
	rpcCtrlMsgAsyncProcessingTimeHistogram prometheus.Histogram
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
		Buckets:   []float64{1, 2, 3, 4, 5, 6, 7, 8},
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

// OnControlMessageIDsTruncated tracks the number of times a control message was truncated.
// Args:
//
//	messageType: the type of the control message that was truncated
//	diff: the number of message ids truncated.
func (c *GossipSubRpcValidationInspectorMetrics) OnControlMessageIDsTruncated(messageType p2pmsg.ControlMessageType, diff int) {
	c.rpcCtrlMsgTruncation.WithLabelValues(messageType.String()).Observe(float64(diff))
}

// OnIHaveMessageTruncated tracks the number of times an IHave message was truncated.
// Note that this function is called whenever an actual iHave message is truncated with all the message ids that it contains.
// This is different from the OnControlMessageIDsTruncated function which is called whenever some message ids are truncated from a control message.
// Args:
//
//	diff: the number of actual messages truncated.
func (c *GossipSubRpcValidationInspectorMetrics) OnIHaveMessageTruncated(diff int) {
	c.OnControlMessageIDsTruncated(labelIHaveMessage, diff)
}

// OnIWantMessagesReceived tracks the number of IWANT messages received by the node from other nodes on an RPC.
// iWant is a control message that is sent by a node to request a message that it has seen advertised in an iHAVE message.
// Note that each IWANT message can contain multiple message ids, but this function only tracks the number of IWANT messages received, not the number of message ids on each IWANT message.
// Args:
//
//	msgCount: the number of IWANT messages received on an RPC.
func (c *GossipSubRpcValidationInspectorMetrics) OnIWantMessagesReceived(msgCount int) {
	c.receivedIWantMsgCount.Add(float64(msgCount))
}

// OnIWantMessageIDsReceived tracks the number of message ids received by the node from other nodes on an RPC.
// Note: this function is called on each IWANT message received by the node.
// Args:
// - msgIdCount: the number of message ids received on the IWANT message.
func (c *GossipSubRpcValidationInspectorMetrics) OnIWantMessageIDsReceived(msgIdCount int) {
	c.receivedIWantMsgIDsHistogram.Observe(float64(msgIdCount))
}

// OnIHaveMessagesReceived tracks the number of IHAVE messages received by the node from another node on an RPC.
// This function is called once per RPC, and counts the number of iHave messages received on that RPC, not per iHave message.
// An IHAVE message can contain multiple message ids, but this function only tracks the number of IHAVE messages received, not the number of message ids on each IHAVE message.
// iHave is a control message that is sent by a node to another node to indicate that it has a new gossiped messages (the message ids).
// Args:
// - msgCount: the number of IHAVE messages received on an RPC.
func (c *GossipSubRpcValidationInspectorMetrics) OnIHaveMessagesReceived(msgCount int) {
	c.receivedIHaveMsgCount.Add(float64(msgCount))
}

// OnIHaveMessageIDsReceived tracks the number of message ids received by the node from other nodes on an iHave message.
// This function is called on each iHave message received by the node.
// Args:
// - channel: the channel on which the iHave message was received.
// - msgIdCount: the number of message ids received on the iHave message.
func (c *GossipSubRpcValidationInspectorMetrics) OnIHaveMessageIDsReceived(channel string, msgIdCount int) {
	c.receivedIHaveMsgIDsHistogram.WithLabelValues(channel).Observe(float64(msgIdCount))
}

// OnGraftReceived tracks the number of GRAFT messages received by the node from another node on an RPC.
// GRAFT is a control message of GossipSub protocol that connects two nodes over a topic directly as gossip partners.
// Note: this function is called once per RPC, and counts the number of graft messages received on that RPC, not per graft message.
func (c *GossipSubRpcValidationInspectorMetrics) OnGraftReceived(count int) {
	c.receivedGraftCount.Add(float64(count))
}

// OnPruneReceived tracks the number of PRUNE messages received by the node from another node on an RPC.
// PRUNE is a control message of GossipSub protocol that disconnects two nodes over a topic.
// Note: this function is called once per RPC, and counts the number of prune messages received on that RPC, not per prune message.
func (c *GossipSubRpcValidationInspectorMetrics) OnPruneReceived(count int) {
	c.receivedPruneCount.Add(float64(count))
}

// OnPublishedGossipMessagesReceived tracks the number of publish messages received by the node from another node on an RPC.
// Args:
// - msgCount: the number of publish messages received on an RPC.
// Note: this function is called once per RPC, and counts the number of publish messages received on that RPC, not per publish message.
func (c *GossipSubRpcValidationInspectorMetrics) OnPublishedGossipMessagesReceived(msgCount int) {
	c.receivedPublishMessageCount.Add(float64(msgCount))
}

// OnIncomingRpcReceived tracks the number of incoming RPC messages received by the node.
func (c *GossipSubRpcValidationInspectorMetrics) OnIncomingRpcReceived() {
	c.incomingRpcCount.Inc()
}
