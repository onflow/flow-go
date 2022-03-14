package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/model/flow"
)

type CacheCollector struct {
	entries   *prometheus.GaugeVec
	hits      *prometheus.CounterVec
	notfounds *prometheus.CounterVec
	misses    *prometheus.CounterVec
}

func NewCacheCollector(chain flow.ChainID) *CacheCollector {

	cm := &CacheCollector{

		entries: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "entries_total",
			Namespace:   namespaceStorage,
			Subsystem:   subsystemCache,
			Help:        "the number of entries in the cache",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}, []string{LabelResource}),

		hits: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "hits_total",
			Namespace:   namespaceStorage,
			Subsystem:   subsystemCache,
			Help:        "the number of hits for the cache",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}, []string{LabelResource}),

		notfounds: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "notfounds_total",
			Namespace:   namespaceStorage,
			Subsystem:   subsystemCache,
			Help:        "the number of times the queried item was not found in either cache or database",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}, []string{LabelResource}),

		misses: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "misses_total",
			Namespace:   namespaceStorage,
			Subsystem:   subsystemCache,
			Help:        "the number of times the queried item was not found in cache, but found in database",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}, []string{LabelResource}),
	}

	return cm
}

// CacheEntries records the size of the node identities cache.
func (cc *CacheCollector) CacheEntries(resource string, entries uint) {
	cc.entries.With(prometheus.Labels{LabelResource: resource}).Set(float64(entries))
}

// CacheHit records the number of hits in the node identities cache.
func (cc *CacheCollector) CacheHit(resource string) {
	cc.hits.With(prometheus.Labels{LabelResource: resource}).Inc()
}

// CacheNotFound records the number of times the queried item was not found in either cache
// or database
func (cc *CacheCollector) CacheNotFound(resource string) {
	cc.notfounds.With(prometheus.Labels{LabelResource: resource}).Inc()
}

// CacheMiss report the number of times the queried item is not found in the cache, but found in the database.
func (cc *CacheCollector) CacheMiss(resource string) {
	cc.misses.With(prometheus.Labels{LabelResource: resource}).Inc()
}
