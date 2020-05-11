package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type BadgerCollector struct {
	lsmSize         prometheus.Gauge
	vlogSize        prometheus.Gauge
	numReads        prometheus.Gauge
	numWrites       prometheus.Gauge
	numBytesRead    prometheus.Gauge
	numBytesWritten prometheus.Gauge
	numGets         prometheus.Gauge
	numPuts         prometheus.Gauge
	numBlockedPuts  prometheus.Gauge
	numMemtableGets prometheus.Gauge
}

func NewBadgerCollector() *BadgerCollector {

	bc := &BadgerCollector{
		lsmSize: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceStorage,
			Subsystem: subsystemBadger,
			Name:      "lsm_bytes",
		}),
		vlogSize: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceStorage,
			Subsystem: subsystemBadger,
			Name:      "vlog_bytes",
		}),
		numReads: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceStorage,
			Subsystem: subsystemBadger,
			Name:      "num_reads",
		}),
		numWrites: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceStorage,
			Subsystem: subsystemBadger,
			Name:      "num_writes",
		}),
		numBytesRead: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceStorage,
			Subsystem: subsystemBadger,
			Name:      "num_bytes_read",
		}),
		numBytesWritten: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceStorage,
			Subsystem: subsystemBadger,
			Name:      "num_bytes_written",
		}),
		numGets: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceStorage,
			Subsystem: subsystemBadger,
			Name:      "num_gets",
		}),
		numPuts: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceStorage,
			Subsystem: subsystemBadger,
			Name:      "num_puts",
		}),
		numBlockedPuts: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceStorage,
			Subsystem: subsystemBadger,
			Name:      "num_blocked_puts",
		}),
		numMemtableGets: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceStorage,
			Subsystem: subsystemBadger,
			Name:      "num_memtable_gets",
		}),
	}

	return bc
}

func (bc *BadgerCollector) BadgerLSMSize(sizeBytes int64) {
	bc.lsmSize.Set(float64(sizeBytes))
}

func (bc *BadgerCollector) BadgerVLogSize(sizeBytes int64) {
	bc.vlogSize.Set(float64(sizeBytes))
}

func (bc *BadgerCollector) BadgerNumReads(n int64) {
	bc.numReads.Set(float64(n))
}

func (bc *BadgerCollector) BadgerNumWrites(n int64) {
	bc.numWrites.Set(float64(n))
}

func (bc *BadgerCollector) BadgerNumBytesRead(n int64) {
	bc.numBytesRead.Set(float64(n))
}

func (bc *BadgerCollector) BadgerNumBytesWritten(n int64) {
	bc.numBytesWritten.Set(float64(n))
}

func (bc *BadgerCollector) BadgerNumGets(n int64) {
	bc.numGets.Set(float64(n))
}

func (bc *BadgerCollector) BadgerNumPuts(n int64) {
	bc.numPuts.Set(float64(n))
}

func (bc *BadgerCollector) BadgerNumBlockedPuts(n int64) {
	bc.numBlockedPuts.Set(float64(n))
}

func (bc *BadgerCollector) BadgerNumMemtableGets(n int64) {
	bc.numMemtableGets.Set(float64(n))
}
