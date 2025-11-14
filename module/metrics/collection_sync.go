package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

type CollectionSyncCollector struct {
	collectionFetchedHeight prometheus.Gauge
	collectionSyncedHeight  prometheus.Gauge
}

var _ module.CollectionSyncMetrics = (*CollectionSyncCollector)(nil)

func NewCollectionSyncCollector() *CollectionSyncCollector {
	return &CollectionSyncCollector{
		collectionFetchedHeight: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "collection_fetched_height",
			Namespace: namespaceAccess,
			Subsystem: "collection_sync",
			Help:      "the highest block height for which collections have been fetched",
		}),
		collectionSyncedHeight: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "collection_synced_height",
			Namespace: namespaceAccess,
			Subsystem: "collection_sync",
			Help:      "the highest block height for which collections have been synced from execution data",
		}),
	}
}

func (c *CollectionSyncCollector) CollectionFetchedHeight(height uint64) {
	c.collectionFetchedHeight.Set(float64(height))
}

func (c *CollectionSyncCollector) CollectionSyncedHeight(height uint64) {
	c.collectionSyncedHeight.Set(float64(height))
}
