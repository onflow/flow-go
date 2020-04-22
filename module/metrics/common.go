package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	badgerDBSizeGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Name:      "badger_db_size_bytes",
	})
	networkMessageSizeGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Subsystem: subsystemNetwork,
		Namespace: namespaceCommon,
		Name:      "message_size_bytes",
		Help:      "size of the outbound network message",
	})
	networkMessageCounter = promauto.NewCounter(prometheus.CounterOpts{
		Subsystem: subsystemNetwork,
		Namespace: namespaceCommon,
		Name:      "message_count",
		Help:      "the number of outbound network messages",
	})
)

// BadgerDBSize sets the total badger database size on disk, measured in bytes.
// This includes the LSM tree and value log.
func (c *Collector) BadgerDBSize(sizeBytes int64) {
	badgerDBSizeGauge.Set(float64(sizeBytes))
}

// NetworkMessageSent increments the message counter and sets the message size of the last message sent out on the wire
// in bytes
func (c *Collector) NetworkMessageSent(sizeBytes int) {
	networkMessageCounter.Inc()
	networkMessageSizeGauge.Set(float64(sizeBytes))
}
