package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// a label for OneToOne messaging for the networking related vector metrics
const TopicLabelOneToOne = "OneToOne"

var (
	badgerDBSizeGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Name:      "badger_db_size_bytes",
	})
	networkOutboundMessageSizeGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: subsystemNetwork,
		Namespace: namespaceCommon,
		Name:      "message_size_bytes_outbound",
		Help:      "size of the outbound network message",
	}, []string{"topic"})
	networkOutboundMessageCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Subsystem: subsystemNetwork,
		Namespace: namespaceCommon,
		Name:      "message_count_outbound",
		Help:      "the number of outbound network messages",
	}, []string{"topic"})
	networkInboundMessageSizeGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: subsystemNetwork,
		Namespace: namespaceCommon,
		Name:      "message_size_bytes_inbound",
		Help:      "size of the inbound network message",
	}, []string{"topic"})
	networkInboundMessageCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Subsystem: subsystemNetwork,
		Namespace: namespaceCommon,
		Name:      "message_count_inbound",
		Help:      "the number of inbound network messages",
	}, []string{"topic"})
)

// BadgerDBSize sets the total badger database size on disk, measured in bytes.
// This includes the LSM tree and value log.
func (c *Collector) BadgerDBSize(sizeBytes int64) {
	badgerDBSizeGauge.Set(float64(sizeBytes))
}

// NetworkMessageSent increments the message counter and sets the message size of the last message sent out on the wire
// in bytes for the given topic
func (c *Collector) NetworkMessageSent(sizeBytes int, topic string) {
	networkOutboundMessageCounter.WithLabelValues(topic).Inc()
	networkOutboundMessageSizeGauge.WithLabelValues(topic).Set(float64(sizeBytes))
}

// NetworkMessageReceived increments the message counter and sets the message size of the last message received on the wire
// in bytes for the given topic
func (c *Collector) NetworkMessageReceived(sizeBytes int, topic string) {
	networkInboundMessageCounter.WithLabelValues(topic).Inc()
	networkInboundMessageSizeGauge.WithLabelValues(topic).Set(float64(sizeBytes))
}
