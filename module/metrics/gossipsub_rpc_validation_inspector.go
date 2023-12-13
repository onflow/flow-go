package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

// GossipSubRpcValidationInspectorMetrics metrics collector for the gossipsub RPC validation inspector.
type GossipSubRpcValidationInspectorMetrics struct {
	prefix                                 string
	rpcCtrlMsgInAsyncPreProcessingGauge    prometheus.Gauge
	rpcCtrlMsgAsyncProcessingTimeHistogram prometheus.Histogram
	invalidCtrlMessageErrorCountHistogram  prometheus.Histogram
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
	gc.invalidCtrlMessageErrorCountHistogram = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gc.prefix + "rpc_invalid_control_message_error_count",
			Help:      "histogram tracking number of errors in a single invalid control message notification",
			Buckets:   []float64{25, 50, 100, 1000},
		},
	)
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

// InvalidControlMessageNotificationErrors tracks the number of errors in each invalid control message notification over time.
func (c *GossipSubRpcValidationInspectorMetrics) InvalidControlMessageNotificationErrors(count int) {
	c.rpcCtrlMsgAsyncProcessingTimeHistogram.Observe(float64(count))
}
