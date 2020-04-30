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
	badgerDBNumReadsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Name:      "badger_db_num_reads",
	})
	badgerDBNumWritesGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Name:      "badger_db_num_writes",
	})
	badgerDBBytesReadGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Name:      "badger_db_read_bytes",
	})
	badgerDBBytesWrittenGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Name:      "badger_db_written_bytes",
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

func (c *Collector) BadgerDBNumReads(numReads int64) {
	badgerDBNumReadsGauge.Set(float64(numReads))
}

func (c *Collector) BadgerDBNumWrites(numWrites int64) {
	badgerDBNumWritesGauge.Set(float64(numWrites))
}

func (c *Collector) BadgerDBNumBytesRead(numBytesRead int64) {
	badgerDBBytesReadGauge.Set(float64(numBytesRead))
}

func (c *Collector) BadgerDBNumBytesWrite(numBytesWrite int64) {
	badgerDBBytesWrittenGauge.Set(float64(numBytesWrite))
}

// NetworkMessageSent increments the message counter and sets the message size of the last message sent out on the wire
// in bytes
func (c *Collector) NetworkMessageSent(sizeBytes int) {
	networkMessageCounter.Inc()
	networkMessageSizeGauge.Set(float64(sizeBytes))
}
