package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// FOLLOWER
	followerFinalizedBlockHeightGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: hotStuffModuleNamespace,
		Subsystem: hotStuffFollowerSubsystem,
		Name:      "finalized_block_height",
	})
)

// FollowerFinalizedBlockHeight
func (c *BaseMetrics) FollowerFinalizedBlockHeight(height uint64) {
	followerFinalizedBlockHeightGauge.Set(float64(height))
}
