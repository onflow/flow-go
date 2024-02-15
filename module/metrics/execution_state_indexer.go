package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

var _ module.ExecutionStateIndexerMetrics = (*ExecutionStateIndexerCollector)(nil)

type ExecutionStateIndexerCollector struct {
	indexDuration        prometheus.Histogram
	highestIndexedHeight prometheus.Gauge

	indexedEvents             prometheus.Counter
	indexedRegisters          prometheus.Counter
	indexedTransactionResults prometheus.Counter
	reindexedHeightCount      prometheus.Counter
}

func NewExecutionStateIndexerCollector() module.ExecutionStateIndexerMetrics {
	indexDuration := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceAccess,
		Subsystem: subsystemExecutionStateIndexer,
		Name:      "index_duration_ms",
		Help:      "the duration of execution state indexing operation",
		Buckets:   []float64{1, 5, 10, 50, 100},
	})

	highestIndexedHeight := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceAccess,
		Subsystem: subsystemExecutionStateIndexer,
		Name:      "highest_indexed_height",
		Help:      "highest block height that has been indexed",
	})

	indexedEvents := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceAccess,
		Subsystem: subsystemExecutionStateIndexer,
		Name:      "indexed_events",
		Help:      "number of events indexed",
	})

	indexedRegisters := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceAccess,
		Subsystem: subsystemExecutionStateIndexer,
		Name:      "indexed_registers",
		Help:      "number of registers indexed",
	})

	indexedTransactionResults := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceAccess,
		Subsystem: subsystemExecutionStateIndexer,
		Name:      "indexed_transaction_results",
		Help:      "number of transaction results indexed",
	})

	reindexedHeightCount := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceAccess,
		Subsystem: subsystemExecutionStateIndexer,
		Name:      "reindexed_height_count",
		Help:      "number of times a previously indexed height is reindexed",
	})

	return &ExecutionStateIndexerCollector{
		indexDuration:             indexDuration,
		highestIndexedHeight:      highestIndexedHeight,
		indexedEvents:             indexedEvents,
		indexedRegisters:          indexedRegisters,
		indexedTransactionResults: indexedTransactionResults,
		reindexedHeightCount:      reindexedHeightCount,
	}
}

// InitializeLatestHeight records the latest height that has been indexed.
// This should only be used during startup. After startup, use BlockIndexed to record newly
// indexed heights.
func (c *ExecutionStateIndexerCollector) InitializeLatestHeight(height uint64) {
	c.highestIndexedHeight.Set(float64(height))
}

// BlockIndexed records metrics from indexing execution data from a single block.
func (c *ExecutionStateIndexerCollector) BlockIndexed(height uint64, duration time.Duration, events, registers, transactionResults int) {
	c.indexDuration.Observe(float64(duration.Milliseconds()))
	c.highestIndexedHeight.Set(float64(height))
	c.indexedEvents.Add(float64(events))
	c.indexedRegisters.Add(float64(registers))
	c.indexedTransactionResults.Add(float64(transactionResults))
}

// BlockReindexed records that a previously indexed block was indexed again.
func (c *ExecutionStateIndexerCollector) BlockReindexed() {
	c.reindexedHeightCount.Inc()
}
