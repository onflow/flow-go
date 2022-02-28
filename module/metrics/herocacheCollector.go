package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
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

func NewHeroCacheCollector(nameSpace string, subSystem string, totalBuckets uint64, registrar prometheus.Registerer) *HeroCacheCollector {

	full := float64(totalBuckets)
	oneFourth := 0.25 * full
	half := 0.5 * full
	threeForth := 0.75 * full

	bucketSlotAvailableHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache + subSystem,
		Buckets:   []float64{oneFourth, half, threeForth, full},
		Name:      "bucket_count_per_available_slot",
		Help:      "histogram of number of buckets with same number of available slots",
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

// BucketsWithAvailableSlotsCountHeroCache keeps track of number of buckets with certain available number of slots.
func (h *HeroCacheCollector) BucketsWithAvailableSlotsCountHeroCache(availableSlots uint64) {
	h.bucketSlotAvailableHistogram.Observe(float64(availableSlots))
}

// OnNewEntityAddedHeroCache is called whenever a new entity is successfully added to the cache.
func (h *HeroCacheCollector) OnNewEntityAddedHeroCache() {
	h.newEntitiesWriteCountTotal.Inc()
}

// OnEntityEjectedAtFullCapacityHeroCache is called whenever adding a new entity to the cache results in ejection of another entity.
// This normally happens when the cache is full.
func (h *HeroCacheCollector) OnEntityEjectedAtFullCapacityHeroCache() {
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
func (h *HeroCacheCollector) Size(size uint64) {
	h.existingEntitiesTotal.Set(float64(size))
}
