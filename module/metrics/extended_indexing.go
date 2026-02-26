package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

var _ module.ExtendedIndexingMetrics = (*ExtendedIndexingCollector)(nil)

type ExtendedIndexingCollector struct {
	indexedHeight            *prometheus.GaugeVec
	scheduledTxCount         *prometheus.CounterVec
	scheduledTxBackfillCount prometheus.Counter
}

func NewExtendedIndexingCollector() module.ExtendedIndexingMetrics {
	indexedHeight := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespaceAccess,
		Subsystem: subsystemExtendedIndexing,
		Name:      "latest_height",
		Help:      "latest processed height for extended indexers",
	}, []string{"indexer"})

	scheduledTxCount := promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespaceAccess,
		Subsystem: subsystemExtendedIndexing,
		Name:      "scheduledtx_total",
		Help:      "total number of scheduled transactions processed, by status",
	}, []string{"status"})

	scheduledTxBackfillCount := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceAccess,
		Subsystem: subsystemExtendedIndexing,
		Name:      "scheduledtx_backfilled_total",
		Help:      "total number of scheduled transactions backfilled from state",
	})

	return &ExtendedIndexingCollector{
		indexedHeight:            indexedHeight,
		scheduledTxCount:         scheduledTxCount,
		scheduledTxBackfillCount: scheduledTxBackfillCount,
	}
}

// BlockIndexedExtended records the latest processed height for a given extended indexer.
func (c *ExtendedIndexingCollector) BlockIndexedExtended(indexer string, height uint64) {
	c.indexedHeight.WithLabelValues(indexer).Set(float64(height))
}

// ScheduledTransactionIndexed records counts of scheduled transactions processed in a single block.
func (c *ExtendedIndexingCollector) ScheduledTransactionIndexed(scheduled, executed, failed, canceled, backfilled int) {
	c.scheduledTxCount.WithLabelValues("scheduled").Add(float64(scheduled))
	c.scheduledTxCount.WithLabelValues("executed").Add(float64(executed))
	c.scheduledTxCount.WithLabelValues("failed").Add(float64(failed))
	c.scheduledTxCount.WithLabelValues("canceled").Add(float64(canceled))
	c.scheduledTxBackfillCount.Add(float64(backfilled))
}
