package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// a label for OneToOne messaging for the networking related vector metrics
const TopicLabelOneToOne = "OneToOne"

const (
	_   = iota
	KiB = 1 << (10 * iota)
	MiB
	GiB
)

var (
	// NETWORKING
	networkOutboundMessageSizeHist = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Subsystem: subsystemNetwork,
		Namespace: namespaceCommon,
		Name:      "outbound_message_size_bytes",
		Help:      "size of the outbound network message",
		Buckets:   []float64{KiB, 100 * KiB, 500 * KiB, 1 * MiB, 2 * MiB, 4 * MiB},
	}, []string{"topic"})
	networkInboundMessageSizeHist = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Subsystem: subsystemNetwork,
		Namespace: namespaceCommon,
		Name:      "inbound_message_size_bytes",
		Help:      "size of the inbound network message",
		Buckets:   []float64{KiB, 100 * KiB, 500 * KiB, 1 * MiB, 2 * MiB, 4 * MiB},
	}, []string{"topic"})
)

// NetworkMessageSent tracks the message size of the last message sent out on the wire
// in bytes for the given topic
func (c *Collector) NetworkMessageSent(sizeBytes int, topic string) {
	networkOutboundMessageSizeHist.WithLabelValues(topic).Observe(float64(sizeBytes))
}

// NetworkMessageReceived tracks the message size of the last message received on the wire
// in bytes for the given topic
func (c *Collector) NetworkMessageReceived(sizeBytes int, topic string) {
	networkInboundMessageSizeHist.WithLabelValues(topic).Observe(float64(sizeBytes))
}
