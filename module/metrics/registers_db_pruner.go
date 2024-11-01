package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

// RegisterDBPrunerCollector collects metrics for the database pruning process, the latest height that has been pruned,
// and the pruning progress.
type RegisterDBPrunerCollector struct {
	lastPrunedHeight prometheus.Gauge // The last pruned block height.
}

var _ module.RegisterDBPrunerMetrics = (*RegisterDBPrunerCollector)(nil)

// NewRegisterDBPrunerCollector creates and returns a new RegisterDBPrunerCollector instance
// with pre-configured Prometheus metrics for monitoring the pruning process.
func NewRegisterDBPrunerCollector() *RegisterDBPrunerCollector {
	return &RegisterDBPrunerCollector{
		lastPrunedHeight: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "last_pruned_height",
			Namespace: namespaceAccess,
			Subsystem: subsystemRegisterDBPruner,
			Help:      "The last block height that has been pruned.",
		}),
	}
}

func (c *RegisterDBPrunerCollector) LatestPrunedHeightWithProgressPercentage(lastPrunedHeight uint64) {
	c.lastPrunedHeight.Set(float64(lastPrunedHeight))
}
