package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type CacheCollector struct {
	entries *prometheus.GaugeVec
	hits    *prometheus.CounterVec
	misses  *prometheus.CounterVec
}

func NewCacheCollector(chain string) *CacheCollector {

	cm := &CacheCollector{

		entries: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "entries_total",
			Namespace:   namespaceStorage,
			Subsystem:   subsystemCache,
			Help:        "the number of entries in the cache",
			ConstLabels: prometheus.Labels{LabelChain: chain},
		}, []string{LabelResource}),

		hits: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "hits_total",
			Namespace:   namespaceStorage,
			Subsystem:   subsystemCache,
			Help:        "the number of hits for the cache",
			ConstLabels: prometheus.Labels{LabelChain: chain},
		}, []string{LabelResource}),

		misses: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "misses_total",
			Namespace:   namespaceStorage,
			Subsystem:   subsystemCache,
			Help:        "the number of misses for the cache",
			ConstLabels: prometheus.Labels{LabelChain: chain},
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

// CacheMiss records the number of misses in the node identities cache.
func (cc *CacheCollector) CacheMiss(resource string) {
	cc.misses.With(prometheus.Labels{LabelResource: resource}).Inc()
}
