package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

// GossipSubRPCValidationInspectorMetrics metrics collector for the gossipsub RPC validation inspector.
type GossipSubRPCValidationInspectorMetrics struct {
	prefix                                  string
	numRpcControlMessagesPreProcessing      *prometheus.GaugeVec
	rpcControlMessagePreProcessingTime      *prometheus.CounterVec
	numRpcIHaveControlMessagesPreProcessing *prometheus.GaugeVec
	rpcIHaveControlMessagePreProcessingTime *prometheus.CounterVec
	numRpcControlMessagesAsyncProcessing    *prometheus.GaugeVec
	rpcControlMessageAsyncProcessTime       *prometheus.CounterVec
}

var _ module.GossipSubRPCValidationInspectorMetrics = (*GossipSubRPCValidationInspectorMetrics)(nil)

// NewGossipSubRPCValidationInspectorMetrics returns a new *GossipSubRPCValidationInspectorMetrics.
func NewGossipSubRPCValidationInspectorMetrics(prefix string) *GossipSubRPCValidationInspectorMetrics {
	gc := &GossipSubRPCValidationInspectorMetrics{prefix: prefix}
	gc.numRpcControlMessagesPreProcessing = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemQueue,
			Name:      gc.prefix + "current_control_messages_preprocessing",
			Help:      "the number of rpc control messages currently being processed",
		}, []string{LabelCtrlMsgType},
	)
	gc.rpcControlMessagePreProcessingTime = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemQueue,
			Name:      gc.prefix + "rpc_control_message_validator_preprocessing_time_seconds",
			Help:      "duration [seconds; measured with float64 precision] of how long the rpc control message validator blocked  pre-processing an rpc control message",
		}, []string{LabelCtrlMsgType},
	)
	gc.numRpcIHaveControlMessagesPreProcessing = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemQueue,
			Name:      gc.prefix + "current_ihave_control_messages_preprocessing",
			Help:      "the number of iHave rpc control messages currently being processed",
		}, []string{LabelCtrlMsgType},
	)
	gc.rpcIHaveControlMessagePreProcessingTime = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemQueue,
			Name:      gc.prefix + "rpc_control_message_validator_ihave_preprocessing_time_seconds",
			Help:      "duration [seconds; measured with float64 precision] of how long the rpc control message validator blocked  pre-processing a sample of iHave control messages",
		}, []string{LabelCtrlMsgType},
	)
	gc.numRpcControlMessagesAsyncProcessing = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemQueue,
			Name:      gc.prefix + "current_control_messages_async_processing",
			Help:      "the number of rpc control messages currently being processed asynchronously by workers from the rpc validator worker pool",
		}, []string{LabelCtrlMsgType},
	)
	gc.rpcControlMessageAsyncProcessTime = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemQueue,
			Name:      gc.prefix + "rpc_control_message_validator_async_processing_time_seconds",
			Help:      "duration [seconds; measured with float64 precision] of how long it takes rpc control message validator to asynchronously process a RPC message",
		}, []string{LabelCtrlMsgType},
	)

	return gc
}

// PreProcessingStarted increments the metric tracking the number of messages being pre-processed by the rpc validation inspector.
func (c *GossipSubRPCValidationInspectorMetrics) PreProcessingStarted(msgType string) {
	c.numRpcControlMessagesPreProcessing.WithLabelValues(msgType).Inc()
}

// PreProcessingFinished tracks the time spent by the rpc validation inspector to pre-process a message and decrements the metric tracking
// the number of messages being processed by the rpc validation inspector.
func (c *GossipSubRPCValidationInspectorMetrics) PreProcessingFinished(msgType string, duration time.Duration) {
	c.numRpcControlMessagesPreProcessing.WithLabelValues(msgType).Dec()
	c.rpcControlMessagePreProcessingTime.WithLabelValues(msgType).Add(duration.Seconds())
}

// IHavePreProcessingStarted increments the metric tracking the number of iHave messages being pre-processed by the rpc validation inspector.
func (c *GossipSubRPCValidationInspectorMetrics) IHavePreProcessingStarted(ihaveMsgType string, sampleSize uint) {
	c.numRpcIHaveControlMessagesPreProcessing.WithLabelValues(ihaveMsgType).Add(float64(sampleSize))
}

// IHavePreProcessingFinished tracks the time spent by the rpc validation inspector to pre-process a iHave message and decrements the metric tracking
// the number of iHave messages being processed by the rpc validation inspector.
func (c *GossipSubRPCValidationInspectorMetrics) IHavePreProcessingFinished(ihaveMsgType string, sampleSize uint, duration time.Duration) {
	c.numRpcIHaveControlMessagesPreProcessing.WithLabelValues(ihaveMsgType).Sub(float64(sampleSize))
	c.rpcIHaveControlMessagePreProcessingTime.WithLabelValues(ihaveMsgType).Add(duration.Seconds())
}

// AsyncProcessingStarted increments the metric tracking the number of messages being processed asynchronously by the rpc validation inspector.
func (c *GossipSubRPCValidationInspectorMetrics) AsyncProcessingStarted(msgType string) {
	c.numRpcControlMessagesAsyncProcessing.WithLabelValues(msgType).Inc()
}

// AsyncProcessingFinished tracks the time spent by the rpc validation inspector to process a message asynchronously and decrements the metric tracking
// the number of messages being processed asynchronously by the rpc validation inspector.
func (c *GossipSubRPCValidationInspectorMetrics) AsyncProcessingFinished(msgType string, duration time.Duration) {
	c.numRpcControlMessagesAsyncProcessing.WithLabelValues(msgType).Dec()
	c.rpcControlMessageAsyncProcessTime.WithLabelValues(msgType).Add(duration.Seconds())
}
