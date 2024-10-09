package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

// RegisterDBPrunerCollector collects metrics for the database pruning process,
// including the durations of pruning operations, the latest height that has been pruned,
// the number of blocks pruned, the number of rows pruned, and the number of elements visited.
type RegisterDBPrunerCollector struct {
	pruneDurations          prometheus.Summary // The pruning operation durations in milliseconds.
	latestHeightPruned      prometheus.Gauge   // The latest pruned block height.
	numberOfBlocksPruned    prometheus.Gauge   // The number of blocks pruned.
	numberOfRowsPruned      prometheus.Gauge   // The number of rows pruned.
	numberOfElementsVisited prometheus.Gauge   // The number of elements visited during pruning.
}

var _ module.RegisterDBPrunerMetrics = (*RegisterDBPrunerCollector)(nil)

// NewRegisterDBPrunerCollector creates and returns a new RegisterDBPrunerCollector instance
// with pre-configured Prometheus metrics for monitoring the pruning process.
func NewRegisterDBPrunerCollector() *RegisterDBPrunerCollector {
	return &RegisterDBPrunerCollector{
		pruneDurations: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "prune_durations_ms",
			Namespace: namespaceAccess,
			Subsystem: subsystemRegisterDBPruner,
			Help:      "The durations of pruning operations in milliseconds.",
			Objectives: map[float64]float64{
				0.01: 0.001,
				0.1:  0.01,
				0.25: 0.025,
				0.5:  0.05,
				0.75: 0.025,
				0.9:  0.01,
				0.99: 0.001,
			},
		}),
		latestHeightPruned: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "latest_height_pruned",
			Namespace: namespaceAccess,
			Subsystem: subsystemRegisterDBPruner,
			Help:      "The latest block height that has been pruned.",
		}),
		numberOfBlocksPruned: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "number_of_blocks_pruned_total",
			Namespace: namespaceAccess,
			Subsystem: subsystemRegisterDBPruner,
			Help:      "The number of blocks that have been pruned.",
		}),
		numberOfRowsPruned: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "number_of_rows_pruned_total",
			Namespace: namespaceAccess,
			Subsystem: subsystemRegisterDBPruner,
			Help:      "The number of rows that have been pruned.",
		}),
		numberOfElementsVisited: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "number_of_elements_visited_total",
			Namespace: namespaceAccess,
			Subsystem: subsystemRegisterDBPruner,
			Help:      "The number of elements that have been visited.",
		}),
	}
}

// Pruned records the duration of a pruning operation and updates the latest pruned height.
//
// Parameters:
// - height: The height of the block that was pruned.
// - duration: The time taken to complete the pruning operation.
func (c *RegisterDBPrunerCollector) Pruned(height uint64, duration time.Duration) {
	c.pruneDurations.Observe(float64(duration.Milliseconds()))
	c.latestHeightPruned.Set(float64(height))
}

// NumberOfRowsPruned records the number of rows that were pruned during the pruning operation.
//
// Parameters:
// - rows: The number of rows that were pruned.
func (c *RegisterDBPrunerCollector) NumberOfRowsPruned(rows uint64) {
	c.numberOfRowsPruned.Add(float64(rows))
}

// ElementVisited records the element that were visited during the pruning operation.
func (c *RegisterDBPrunerCollector) ElementVisited() {
	c.numberOfElementsVisited.Inc()
}

// NumberOfBlocksPruned tracks the number of blocks that were pruned during the operation.
//
// Parameters:
// - blocks: The number of blocks that were pruned.
func (c *RegisterDBPrunerCollector) NumberOfBlocksPruned(blocks uint64) {
	c.numberOfBlocksPruned.Add(float64(blocks))
}
