package metrics

import (
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
