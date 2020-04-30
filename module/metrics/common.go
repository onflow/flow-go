package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// a label for OneToOne messaging for the networking related vector metrics
const TopicLabelOneToOne = "OneToOne"

var (
	// BADGER
	badgerLSMSizeGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Subsystem: subsystemBadger,
		Name:      "db_lsm_bytes",
	})
	badgerVLogSizeGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Subsystem: subsystemBadger,
		Name:      "db_vlog_bytes",
	})
	badgerDBNumReads = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Subsystem: subsystemBadger,
		Name:      "db_num_reads",
	})
	badgerDBNumWrites = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Subsystem: subsystemBadger,
		Name:      "db_num_writes",
	})
	badgerDBNumBytesRead = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Subsystem: subsystemBadger,
		Name:      "db_num_bytes_read",
	})
	badgerDBNumBytesWritten = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Subsystem: subsystemBadger,
		Name:      "db_num_bytes_written",
	})
	badgerDBNumGets = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Subsystem: subsystemBadger,
		Name:      "db_num_gets",
	})
	badgerDBNumPuts = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Subsystem: subsystemBadger,
		Name:      "db_num_puts",
	})
	badgerDBNumBlockedPuts = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Subsystem: subsystemBadger,
		Name:      "db_num_blocked_puts",
	})
	badgerDBNumMemtableGets = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Subsystem: subsystemBadger,
		Name:      "db_num_memtable_gets",
	})

	// NETWORKING
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

// Badger DB size can be calculated by LSM plus VLog size
// BadgerLSMSize
func (c *Collector) BadgerLSMSize(sizeBytes int64) {
	badgerLSMSizeGauge.Set(float64(sizeBytes))
}

func (c *Collector) BadgerVLogSize(sizeBytes int64) {
	badgerVLogSizeGauge.Set(float64(sizeBytes))
}

func (c *Collector) BadgerNumReads(n int64) {
	badgerDBNumReads.Set(float64(n))
}

func (c *Collector) BadgerNumWrites(n int64) {
	badgerDBNumWrites.Set(float64(n))
}

func (c *Collector) BadgerNumBytesRead(n int64) {
	badgerDBNumBytesRead.Set(float64(n))
}

func (c *Collector) BadgerNumBytesWritten(n int64) {
	badgerDBNumBytesWritten.Set(float64(n))
}

func (c *Collector) BadgerNumGets(n int64) {
	badgerDBNumGets.Set(float64(n))
}

func (c *Collector) BadgerNumPuts(n int64) {
	badgerDBNumPuts.Set(float64(n))
}

func (c *Collector) BadgerNumBlockedPuts(n int64) {
	badgerDBNumBlockedPuts.Set(float64(n))
}

func (c *Collector) BadgerNumMemtableGets(n int64) {
	badgerDBNumMemtableGets.Set(float64(n))
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
