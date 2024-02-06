package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

// GossipSubScoringRegistryMetrics encapsulates the metrics collectors for collecting metrics related to the Gossipsub scoring registry, offering insights into penalties and
// other factors used by the scoring registry to compute the application-specific score. It focuses on tracking internal
// aspects of the application-specific score, distinguishing itself from GossipSubScoringMetrics.
type GossipSubScoringRegistryMetrics struct {
	prefix                    string
	duplicateMessagePenalties prometheus.Histogram
	duplicateMessageCounts    prometheus.Histogram
}

var _ module.GossipSubScoringRegistryMetrics = (*GossipSubScoringRegistryMetrics)(nil)

// NewGossipSubScoringRegistryMetrics returns a new *GossipSubScoringRegistryMetrics.
func NewGossipSubScoringRegistryMetrics(prefix string) *GossipSubScoringRegistryMetrics {
	gc := &GossipSubScoringRegistryMetrics{prefix: prefix}
	gc.duplicateMessagePenalties = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gc.prefix + "gossipsub_scoring_registry_duplicate_message_penalties",
			Help:      "duplicate message penalty applied to the overall application specific score of a node",
			Buckets:   []float64{-1, -0.01, -0.001},
		},
	)
	gc.duplicateMessageCounts = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      gc.prefix + "gossipsub_scoring_registry_duplicate_message_counts",
			Help:      "duplicate message count of a node at the time it is used to compute the duplicate message penalty",
			Buckets:   []float64{25, 50, 100, 1000},
		},
	)
	return gc
}

// DuplicateMessagePenalties tracks the duplicate message penalty for a node.
func (g GossipSubScoringRegistryMetrics) DuplicateMessagePenalties(penalty float64) {
	g.duplicateMessagePenalties.Observe(penalty)
}

// DuplicateMessagesCounts tracks the duplicate message count for a node.
func (g GossipSubScoringRegistryMetrics) DuplicateMessagesCounts(count float64) {
	g.duplicateMessageCounts.Observe(count)
}
