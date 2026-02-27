package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

var _ module.ExtendedIndexingMetrics = (*ExtendedIndexingCollector)(nil)

type ExtendedIndexingCollector struct {
	indexedHeight *prometheus.GaugeVec
}

func NewExtendedIndexingCollector() module.ExtendedIndexingMetrics {
	indexedHeight := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespaceAccess,
		Subsystem: subsystemExtendedIndexing,
		Name:      "latest_height",
		Help:      "latest processed height for extended indexers",
	}, []string{"indexer"})

	return &ExtendedIndexingCollector{
		indexedHeight: indexedHeight,
	}
}

// BlockIndexedExtended records the latest processed height for a given extended indexer.
func (c *ExtendedIndexingCollector) BlockIndexedExtended(indexer string, height uint64) {
	c.indexedHeight.WithLabelValues(indexer).Set(float64(height))
}
