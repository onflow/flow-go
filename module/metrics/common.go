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

// Badger DB size can be calculated by LSM plus VLog size
// BadgerLSMSize
func (c *BaseMetrics) BadgerLSMSize(sizeBytes int64) {
	badgerLSMSizeGauge.Set(float64(sizeBytes))
}

func (c *BaseMetrics) BadgerVLogSize(sizeBytes int64) {
	badgerVLogSizeGauge.Set(float64(sizeBytes))
}

func (c *BaseMetrics) BadgerNumReads(n int64) {
	badgerDBNumReads.Set(float64(n))
}

func (c *BaseMetrics) BadgerNumWrites(n int64) {
	badgerDBNumWrites.Set(float64(n))
}

func (c *BaseMetrics) BadgerNumBytesRead(n int64) {
	badgerDBNumBytesRead.Set(float64(n))
}

func (c *BaseMetrics) BadgerNumBytesWritten(n int64) {
	badgerDBNumBytesWritten.Set(float64(n))
}

func (c *BaseMetrics) BadgerNumGets(n int64) {
	badgerDBNumGets.Set(float64(n))
}

func (c *BaseMetrics) BadgerNumPuts(n int64) {
	badgerDBNumPuts.Set(float64(n))
}

func (c *BaseMetrics) BadgerNumBlockedPuts(n int64) {
	badgerDBNumBlockedPuts.Set(float64(n))
}

func (c *BaseMetrics) BadgerNumMemtableGets(n int64) {
	badgerDBNumMemtableGets.Set(float64(n))
}

// NetworkMessageSent tracks the message size of the last message sent out on the wire
// in bytes for the given topic
func (c *BaseMetrics) NetworkMessageSent(sizeBytes int, topic string) {
	networkOutboundMessageSizeHist.WithLabelValues(topic).Observe(float64(sizeBytes))
}

// NetworkMessageReceived tracks the message size of the last message received on the wire
// in bytes for the given topic
func (c *BaseMetrics) NetworkMessageReceived(sizeBytes int, topic string) {
	networkInboundMessageSizeHist.WithLabelValues(topic).Observe(float64(sizeBytes))
}
