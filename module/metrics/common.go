package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	badgerDBSizeKB = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceCommon,
		Name:      "badger_db_size_bytes",
	})
)

// BadgerDBSize sets the total badger database size on disk, measured in bytes.
// This includes the LSM tree and value log.
func (c *Collector) BadgerDBSize(sizeBytes int64) {
	badgerDBSizeKB.Set(float64(sizeBytes))
}
