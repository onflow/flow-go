package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	_   = iota
	KiB = 1 << (10 * iota)
	MiB
	GiB
)

type NetworkCollector struct {
	outboundMessageSize      *prometheus.HistogramVec
	inboundMessageSize       *prometheus.HistogramVec
	duplicateMessagesDropped *prometheus.CounterVec
	queueSize                *prometheus.GaugeVec
	queueDuration            *prometheus.HistogramVec
}

func NewNetworkCollector() *NetworkCollector {

	nc := &NetworkCollector{

		outboundMessageSize: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      "outbound_message_size_bytes",
			Help:      "size of the outbound network message",
			Buckets:   []float64{KiB, 100 * KiB, 500 * KiB, 1 * MiB, 2 * MiB, 4 * MiB},
		}, []string{LabelChannel}),

		inboundMessageSize: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      "inbound_message_size_bytes",
			Help:      "size of the inbound network message",
			Buckets:   []float64{KiB, 100 * KiB, 500 * KiB, 1 * MiB, 2 * MiB, 4 * MiB},
		}, []string{LabelChannel}),

		duplicateMessagesDropped: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      "duplicate_messages_dropped",
			Help:      "number of duplicate messages dropped",
		}, []string{LabelChannel}),

		queueSize: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemQueue,
			Help:      "the number of elements in the message receive queue",
		}, []string{LabelPriority}),

		queueDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:      "queue_duration_seconds",
			Namespace: namespaceNetwork,
			Subsystem: subsystemHotstuff,
			Help:      "duration [millis; measured with float64 precision] of how long a message spent in the queue before delivered to an engine.",
			Buckets:   []float64{10, 100, 500, 1000, 2000, 5000},
		}, []string{LabelPriority}),
	}

	return nc
}

// NetworkMessageSent tracks the message size of the last message sent out on the wire
// in bytes for the given topic
func (nc *NetworkCollector) NetworkMessageSent(sizeBytes int, topic string) {
	nc.outboundMessageSize.WithLabelValues(topic).Observe(float64(sizeBytes))
}

// NetworkMessageReceived tracks the message size of the last message received on the wire
// in bytes for the given topic
func (nc *NetworkCollector) NetworkMessageReceived(sizeBytes int, topic string) {
	nc.inboundMessageSize.WithLabelValues(topic).Observe(float64(sizeBytes))
}

// NetworkDuplicateMessagesDropped tracks the number of messages dropped by the network layer due to duplication
func (nc *NetworkCollector) NetworkDuplicateMessagesDropped(topic string) {
	nc.duplicateMessagesDropped.WithLabelValues(topic).Add(1)
}

func (nc *NetworkCollector) ElementAdded(priority string) {
	nc.queueSize.WithLabelValues(priority).Inc()
}

func (nc *NetworkCollector) ElementRemoved(priority string) {
	nc.queueSize.WithLabelValues(priority).Dec()
}

func (nc *NetworkCollector) QueueDuration(duration time.Duration, priority string) {
	nc.queueDuration.WithLabelValues(priority).Observe(float64(duration.Milliseconds()))
}
