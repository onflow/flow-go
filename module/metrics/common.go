package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// BADGER
	badgerDBSizeGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Subsystem: subsystemBadger,
		Name:      "db_size_bytes",
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
	badgerDBNumWrites.Set(float64(n))
}

func (c *Collector) BadgerNumBlockedPuts(n int64) {
	badgerDBNumBlockedPuts.Set(float64(n))
}

func (c *Collector) BadgerNumMemtableGets(n int64) {
	badgerDBNumMemtableGets.Set(float64(n))
}

// NetworkMessageSent increments the message counter and sets the message size of the last message sent out on the wire
// in bytes
func (c *Collector) NetworkMessageSent(sizeBytes int) {
	networkMessageCounter.Inc()
	networkMessageSizeGauge.Set(float64(sizeBytes))
}
