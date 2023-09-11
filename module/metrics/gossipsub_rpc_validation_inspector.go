package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

// GossipSubRpcValidationInspectorMetrics metrics collector for the gossipsub RPC validation inspector.
type GossipSubRpcValidationInspectorMetrics struct {
	prefix                                    string
	rpcCtrlMsgInBlockingPreProcessingGauge    *prometheus.GaugeVec
	rpcCtrlMsgBlockingProcessingTimeHistogram *prometheus.HistogramVec
	rpcCtrlMsgInAsyncPreProcessingGauge       *prometheus.GaugeVec
	rpcCtrlMsgAsyncProcessingTimeHistogram    *prometheus.HistogramVec
}

var _ module.GossipSubRpcValidationInspectorMetrics = (*GossipSubRpcValidationInspectorMetrics)(nil)

// NewGossipSubRPCValidationInspectorMetrics returns a new *GossipSubRpcValidationInspectorMetrics.
func NewGossipSubRPCValidationInspectorMetrics(prefix string) *GossipSubRpcValidationInspectorMetrics {
	gc := &GossipSubRpcValidationInspectorMetrics{prefix: prefix}
	gc.rpcCtrlMsgInBlockingPreProcessingGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gc.prefix + "control_message_in_blocking_preprocess_total",
			Help:      "the number of rpc control messages currently being pre-processed",
		}, []string{LabelCtrlMsgType},
	)
	gc.rpcCtrlMsgBlockingProcessingTimeHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gc.prefix + "rpc_control_message_validator_blocking_preprocessing_time_seconds",
			Help:      "duration [seconds; measured with float64 precision] of how long the rpc control message validator blocked  pre-processing an rpc control message",
			Buckets:   []float64{.1, .25, .5, 1, 2.5, 5, 7.5, 10, 20},
		}, []string{LabelCtrlMsgType},
	)
	gc.rpcCtrlMsgInAsyncPreProcessingGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gc.prefix + "control_messages_in_async_processing_total",
			Help:      "the number of rpc control messages currently being processed asynchronously by workers from the rpc validator worker pool",
		}, []string{LabelCtrlMsgType},
	)
	gc.rpcCtrlMsgAsyncProcessingTimeHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gc.prefix + "rpc_control_message_validator_async_processing_time_seconds",
			Help:      "duration [seconds; measured with float64 precision] of how long it takes rpc control message validator to asynchronously process a rpc message",
			Buckets:   []float64{.1, .25, .5, 1, 2.5, 5, 7.5, 10, 20},
		}, []string{LabelCtrlMsgType},
	)

	return gc
}

// BlockingPreProcessingStarted increments the metric tracking the number of messages being pre-processed by the rpc validation inspector.
func (c *GossipSubRpcValidationInspectorMetrics) BlockingPreProcessingStarted(msgType string, sampleSize uint) {
	c.rpcCtrlMsgInBlockingPreProcessingGauge.WithLabelValues(msgType).Add(float64(sampleSize))
}

// BlockingPreProcessingFinished tracks the time spent by the rpc validation inspector to pre-process a message and decrements the metric tracking
// the number of messages being processed by the rpc validation inspector.
func (c *GossipSubRpcValidationInspectorMetrics) BlockingPreProcessingFinished(msgType string, sampleSize uint, duration time.Duration) {
	c.rpcCtrlMsgInBlockingPreProcessingGauge.WithLabelValues(msgType).Sub(float64(sampleSize))
	c.rpcCtrlMsgBlockingProcessingTimeHistogram.WithLabelValues(msgType).Observe(duration.Seconds())
}

// AsyncProcessingStarted increments the metric tracking the number of messages being processed asynchronously by the rpc validation inspector.
func (c *GossipSubRpcValidationInspectorMetrics) AsyncProcessingStarted(msgType string) {
	c.rpcCtrlMsgInAsyncPreProcessingGauge.WithLabelValues(msgType).Inc()
}

// AsyncProcessingFinished tracks the time spent by the rpc validation inspector to process a message asynchronously and decrements the metric tracking
// the number of messages being processed asynchronously by the rpc validation inspector.
func (c *GossipSubRpcValidationInspectorMetrics) AsyncProcessingFinished(msgType string, duration time.Duration) {
	c.rpcCtrlMsgInAsyncPreProcessingGauge.WithLabelValues(msgType).Dec()
	c.rpcCtrlMsgAsyncProcessingTimeHistogram.WithLabelValues(msgType).Observe(duration.Seconds())
}
