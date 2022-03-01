package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/module"
)

const subsystemHeroCache = "hero_cache"

type HeroCacheCollector struct {
	bucketSlotAvailableHistogram     prometheus.Histogram
	newEntitiesWriteCountTotal       prometheus.Counter
	entityEjectedAtFullCapacityTotal prometheus.Counter
	fullBucketsFoundTimes            prometheus.Counter
	duplicateWriteQueriesTotal       prometheus.Counter
}

type HeroCacheMetricsRegistrationFunc func(uint64) module.HeroCacheMetrics

func NetworkReceiveCacheMetricsFactory(registrar prometheus.Registerer) HeroCacheMetricsRegistrationFunc {
	return func(totalBuckets uint64) module.HeroCacheMetrics {
		return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingReceiveCache, totalBuckets, registrar)
	}
}

func NetworkDnsCacheMetricsFactory(registrar prometheus.Registerer) HeroCacheMetricsRegistrationFunc {
	return func(totalBuckets uint64) module.HeroCacheMetrics {
		return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingDnsCache, totalBuckets, registrar)
	}
}

func CollectionNodeTransactionsCacheMetricsFactory(registrar prometheus.Registerer) HeroCacheMetricsRegistrationFunc {
	return func(totalBuckets uint64) module.HeroCacheMetrics {
		return NewHeroCacheCollector(namespaceCollection, ResourceTransaction, totalBuckets, registrar)
	}
}

func NewHeroCacheCollector(nameSpace string, cacheName string, totalBuckets uint64, registrar prometheus.Registerer) *HeroCacheCollector {

	hundredPercent := float64(totalBuckets)
	tenPercent := 0.1 * hundredPercent
	twentyPercent := 0.25 * hundredPercent
	fiftyPercent := 0.5 * hundredPercent
	seventyFivePercent := 0.75 * hundredPercent

	bucketSlotAvailableHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Buckets:   []float64{1, 2, tenPercent, twentyPercent, fiftyPercent, seventyFivePercent, hundredPercent},
		Name:      cacheName + "_" + "bucket_available_slot_count",
		Help:      "histogram of number of available slots in buckets of cache",
	})

	newEntitiesWriteCountTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "new_entity_write_count_total",
		Help:      "total number of writes to the hero cache that carry a new unique entity (non-duplicate)",
	})

	entityEjectedAtFullCapacityTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "entities_ejected_at_full_capacity_total",
		Help:      "total number of entities ejected when writing new entities at full capacity",
	})

	fullBucketFoundTimes := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "full_bucket_found_times_total",
		Help:      "total times adding a new entity faces a full bucket",
	})

	duplicateWriteQueriesTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "duplicate_write_queries_total",
		Help:      "total number of queries writing an already existing (duplicate) entity to the cache",
	})

	existingEntitiesTotal := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "size_entities_total",
		Help:      "total number of entities maintained on the cache",
	})

	registrar.MustRegister(
		bucketSlotAvailableHistogram,
		newEntitiesWriteCountTotal,
		entityEjectedAtFullCapacityTotal,
		fullBucketFoundTimes,
		duplicateWriteQueriesTotal,
		existingEntitiesTotal)

	return &HeroCacheCollector{
		bucketSlotAvailableHistogram:     bucketSlotAvailableHistogram,
		newEntitiesWriteCountTotal:       newEntitiesWriteCountTotal,
		entityEjectedAtFullCapacityTotal: entityEjectedAtFullCapacityTotal,
		fullBucketsFoundTimes:            fullBucketFoundTimes,
		duplicateWriteQueriesTotal:       duplicateWriteQueriesTotal,
	}
}

// BucketAvailableSlotsCount keeps track of number of available slots in buckets of cache.
func (h *HeroCacheCollector) BucketAvailableSlotsCount(availableSlots uint64) {
	h.bucketSlotAvailableHistogram.Observe(float64(availableSlots))
}

// OnNewEntityAdded is called whenever a new entity is successfully added to the cache.
func (h *HeroCacheCollector) OnNewEntityAdded() {
	h.newEntitiesWriteCountTotal.Inc()
}

// OnEntityEjectedAtFullCapacityHeroCache is called whenever adding a new entity to the cache results in ejection of another entity.
// This normally happens when the cache is full.
func (h *HeroCacheCollector) OnEntityEjectedAtFullCapacity() {
	h.entityEjectedAtFullCapacityTotal.Inc()
}

// OnBucketFull is called whenever a bucket is found full and all of its keys are valid.
// Hence, adding a new entity to that bucket will replace the oldest valid key inside that bucket.
func (h *HeroCacheCollector) OnBucketFull() {
	h.fullBucketsFoundTimes.Inc()
}

// OnDuplicateWriteQuery is tracking the total number of duplicate entities written to the cache. A duplicate entity is dropped by the
// cache when it is written to the cache, and this metric is tracking the total number of those queries.
func (h *HeroCacheCollector) OnDuplicateWriteQuery() {
	h.duplicateWriteQueriesTotal.Inc()
}
