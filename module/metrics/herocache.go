package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/module"
)

const subsystemHeroCache = "hero_cache_"

type HeroCacheCollector struct {
	bucketSlotAvailableHistogram     prometheus.Histogram
	newEntitiesWriteCountTotal       prometheus.Counter
	entityEjectedAtFullCapacityTotal prometheus.Counter
	validKeyReplacedTotal            prometheus.Counter
	duplicateWriteQueriesTotal       prometheus.Counter
	existingEntitiesTotal            prometheus.Gauge
}

type HeroCacheMetricsRegistrationFunc func(uint64) module.HeroCacheMetrics

func NetworkReceiveCacheMetricsFactory(registrar prometheus.Registerer) HeroCacheMetricsRegistrationFunc {
	return func(totalBuckets uint64) module.HeroCacheMetrics {
		return NewHeroCacheCollector(namespaceNetwork, subsystemReceiveCache, totalBuckets, registrar)
	}
}

func NetworkDnsCacheMetricsFactory(registrar prometheus.Registerer) HeroCacheMetricsRegistrationFunc {
	return func(totalBuckets uint64) module.HeroCacheMetrics {
		return NewHeroCacheCollector(namespaceNetwork, subsystemDnsCache, totalBuckets, registrar)
	}
}

func CollectionNodeTransactionsCacheMetricsFactory(registrar prometheus.Registerer) HeroCacheMetricsRegistrationFunc {
	return func(totalBuckets uint64) module.HeroCacheMetrics {
		return NewHeroCacheCollector(namespaceCollection, subsystemTransactionsCache, totalBuckets, registrar)
	}
}

func NewHeroCacheCollector(nameSpace string, subSystem string, totalBuckets uint64, registrar prometheus.Registerer) *HeroCacheCollector {

	hundredPercent := float64(totalBuckets)
	tenPercent := 0.1 * hundredPercent
	twentyPercent := 0.25 * hundredPercent
	fiftyPercent := 0.5 * hundredPercent
	seventyFivePercent := 0.75 * hundredPercent

	bucketSlotAvailableHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache + subSystem,
		Buckets:   []float64{1, 2, tenPercent, twentyPercent, fiftyPercent, seventyFivePercent, hundredPercent},
		Name:      "bucket_available_slot_count",
		Help:      "histogram of number of available slots in buckets of cache",
	})

	newEntitiesWriteCountTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache + subSystem,
		Name:      "new_entity_write_count_total",
		Help:      "total number of writes to the hero cache that carry a new unique entity (non-duplicate)",
	})

	entityEjectedAtFullCapacityTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache + subSystem,
		Name:      "entities_ejected_at_full_capacity_total",
		Help:      "total number of entities ejected when writing new entities at full capacity",
	})

	validKeyReplacedTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache + subSystem,
		Name:      "valid_keys_replaced_total",
		Help:      "total number of valid keys replaced on full buckets when adding a new entity",
	})

	duplicateWriteQueriesTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache + subSystem,
		Name:      "duplicate_write_queries_total",
		Help:      "total number of queries writing an already existing (duplicate) entity to the cache",
	})

	existingEntitiesTotal := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache + subSystem,
		Name:      "size_entities_total",
		Help:      "total number of entities maintained on the cache",
	})

	registrar.MustRegister(
		bucketSlotAvailableHistogram,
		newEntitiesWriteCountTotal,
		entityEjectedAtFullCapacityTotal,
		validKeyReplacedTotal,
		duplicateWriteQueriesTotal,
		existingEntitiesTotal)

	return &HeroCacheCollector{
		bucketSlotAvailableHistogram:     bucketSlotAvailableHistogram,
		newEntitiesWriteCountTotal:       newEntitiesWriteCountTotal,
		entityEjectedAtFullCapacityTotal: entityEjectedAtFullCapacityTotal,
		validKeyReplacedTotal:            validKeyReplacedTotal,
		duplicateWriteQueriesTotal:       duplicateWriteQueriesTotal,
		existingEntitiesTotal:            existingEntitiesTotal,
	}
}

// BucketAvailableSlotsCountHeroCache keeps track of number of available slots in buckets of cache.
func (h *HeroCacheCollector) BucketAvailableSlotsCountHeroCache(availableSlots uint64) {
	h.bucketSlotAvailableHistogram.Observe(float64(availableSlots))
}

// OnNewEntityAddedHeroCache is called whenever a new entity is successfully added to the cache.
func (h *HeroCacheCollector) OnNewEntityAddedHeroCache() {
	h.newEntitiesWriteCountTotal.Inc()
}

// OnEntityEjectedAtFullCapacityHeroCache is called whenever adding a new entity to the cache results in ejection of another entity.
// This normally happens when the cache is full.
func (h *HeroCacheCollector) OnEntityEjectedAtFull() {
	h.entityEjectedAtFullCapacityTotal.Inc()
}

// OnValidKeyReplaced is called whenever adding a new entity to the cache results in a valid existing key replaced. This happens when
// a bucket of keys gets full. Then adding any new entity will replace a valid key inside that bucket.
func (h *HeroCacheCollector) OnValidKeyReplaced() {
	h.validKeyReplacedTotal.Inc()
}

// OnDuplicateWriteQuery is tracking the total number of duplicate entities written to the cache. A duplicate entity is dropped by the
// cache when it is written to the cache, and this metric is tracking the total number of those queries.
func (h *HeroCacheCollector) OnDuplicateWriteQuery() {
	h.duplicateWriteQueriesTotal.Inc()
}

// Size keeps track of the total number of entities maintained by the cache.
func (h *HeroCacheCollector) Size(size uint32) {
	h.existingEntitiesTotal.Set(float64(size))
}
