package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

// RegisterDBPrunerCollector collects metrics for the database pruning process,
// including the durations of pruning operations and the latest height that has been pruned.
type RegisterDBPrunerCollector struct {
	pruneDurations     prometheus.Summary // Summary metric for tracking pruning operation durations in milliseconds.
	latestHeightPruned prometheus.Gauge   // Gauge metric for tracking the latest pruned block height.
}

var _ module.RegisterDBPrunerMetrics = (*AccessCollector)(nil)

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
