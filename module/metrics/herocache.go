package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/module"
)

const subsystemHeroCache = "hero_cache"

type HeroCacheCollector struct {
	bucketSlotAvailableHistogram prometheus.Histogram

	// TODO: successful and unsuccessful reads counters.

	newEntitiesWriteCountTotal prometheus.Counter
	duplicateWriteQueriesTotal prometheus.Counter

	entityEjectedAtFullCapacityTotal prometheus.Counter
	fullBucketsFoundTimes            prometheus.Counter
}

type HeroCacheMetricsRegistrationFunc func(uint64) module.HeroCacheMetrics

func NetworkReceiveCacheMetricsFactory(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingReceiveCache, registrar)
}

func NetworkDnsCacheMetricsFactory(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingDnsCache, registrar)
}

func CollectionNodeTransactionsCacheMetrics(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceCollection, ResourceTransaction, registrar)
}

func NewHeroCacheCollector(nameSpace string, cacheName string, registrar prometheus.Registerer) *HeroCacheCollector {

	bucketSlotAvailableHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Buckets:   []float64{0, 0.05, 0.1, 0.25, 0.5, 0.75, 1},
		Name:      cacheName + "_" + "bucket_available_slot_percent_count",
		Help:      "histogram of buckets vs normalized capacity of available slots (between 0 and 1)",
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

	registrar.MustRegister(
		bucketSlotAvailableHistogram,
		newEntitiesWriteCountTotal,
		entityEjectedAtFullCapacityTotal,
		fullBucketFoundTimes,
		duplicateWriteQueriesTotal)

	return &HeroCacheCollector{
		bucketSlotAvailableHistogram:     bucketSlotAvailableHistogram,
		newEntitiesWriteCountTotal:       newEntitiesWriteCountTotal,
		entityEjectedAtFullCapacityTotal: entityEjectedAtFullCapacityTotal,
		fullBucketsFoundTimes:            fullBucketFoundTimes,
		duplicateWriteQueriesTotal:       duplicateWriteQueriesTotal,
	}
}

// BucketAvailableSlotsCount keeps track of number of available slots in buckets of cache.
func (h *HeroCacheCollector) BucketAvailableSlotsCount(availableSlots uint64, totalSlots uint64) {
	normalizedAvailableSlots := float64(availableSlots) / float64(totalSlots)
	h.bucketSlotAvailableHistogram.Observe(normalizedAvailableSlots)
}

// OnNewEntityAdded is called whenever a new entity is successfully added to the cache.
func (h *HeroCacheCollector) OnNewEntityAdded() {
	h.newEntitiesWriteCountTotal.Inc()
}

// OnEntityEjectedAtFullCapacity is called whenever adding a new entity to the cache results in ejection of another entity.
// This normally happens when the cache is full.
func (h *HeroCacheCollector) OnEntityEjectedAtFullCapacity() {
	h.entityEjectedAtFullCapacityTotal.Inc()
}

// OnBucketFull is called whenever a bucket is found full and all of its keys are valid.
// Hence, adding a new entity to that bucket will replace the oldest valid key inside that bucket.
func (h *HeroCacheCollector) OnBucketFull() {
	h.fullBucketsFoundTimes.Inc()
}

// OnAddingDuplicateEntityAttempt is tracking the total number of attempts on adding a duplicate entity to the cache.
// A duplicate entity is dropped by the cache when it is written to the cache,
// and this metric is tracking the total number of those queries.
func (h *HeroCacheCollector) OnAddingDuplicateEntityAttempt() {
	h.duplicateWriteQueriesTotal.Inc()
}
