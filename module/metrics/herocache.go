package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/module"
)

const subsystemHeroCache = "hero_cache"

type HeroCacheCollector struct {
	histogramNormalizedBucketSlotAvailable prometheus.Histogram

	countKeyGetSuccess prometheus.Counter
	countKeyGetFailure prometheus.Counter

	countKeyPutSuccess prometheus.Counter
	countKeyPutFailure prometheus.Counter

	countKeyEjectionDueToFullCapacity prometheus.Counter
	countKeyEjectionDueToEmergency    prometheus.Counter
}

type HeroCacheMetricsRegistrationFunc func(uint64) module.HeroCacheMetrics

func NetworkReceiveCacheMetricsFactory(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingReceiveCache, registrar)
}

func NetworkDnsTxtCacheMetricsFactory(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingDnsTxtCache, registrar)
}

func NetworkDnsIpCacheMetricsFactory(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingDnsIpCache, registrar)
}

func CollectionNodeTransactionsCacheMetrics(registrar prometheus.Registerer, epoch uint64) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceCollection, fmt.Sprintf("%s_%d", ResourceTransaction, epoch), registrar)
}

func NewHeroCacheCollector(nameSpace string, cacheName string, registrar prometheus.Registerer) *HeroCacheCollector {

	histogramNormalizedBucketSlotAvailable := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Buckets:   []float64{0, 0.05, 0.1, 0.25, 0.5, 0.75, 1},
		Name:      cacheName + "_" + "normalized_bucket_available_slot_count",
		Help:      "normalized histogram of available slots across all buckets",
	})

	countKeyGetSuccess := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "successful_read_count_total",
		Help:      "total number of successful read queries",
	})

	unsuccessfulReadCount := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "unsuccessful_read_count_total",
		Help:      "total number of unsuccessful read queries",
	})

	countKeyPutSuccess := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "successful_write_count_total",
		Help:      "total number successful write queries",
	})

	countKeyPutFailure := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "unsuccessful_write_count_total",
		Help:      "total number of queries writing an already existing (duplicate) entity to the cache",
	})

	countKeyEjectionDueToFullCapacity := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "full_capacity_entity_ejection_total",
		Help:      "total number of entities ejected when writing new entities at full capacity",
	})

	countKeyEjectionDueToEmergency := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "emergency_key_ejection_total",
		Help:      "total number of emergency key ejections at bucket level",
	})

	registrar.MustRegister(
		// available slot distribution
		histogramNormalizedBucketSlotAvailable,

		// read
		countKeyGetSuccess,
		unsuccessfulReadCount,

		// write
		countKeyPutSuccess,
		countKeyPutFailure,

		// ejection
		countKeyEjectionDueToFullCapacity,
		countKeyEjectionDueToEmergency)

	return &HeroCacheCollector{
		histogramNormalizedBucketSlotAvailable: histogramNormalizedBucketSlotAvailable,

		countKeyGetSuccess: countKeyGetSuccess,
		countKeyGetFailure: unsuccessfulReadCount,

		countKeyPutSuccess: countKeyPutSuccess,
		countKeyPutFailure: countKeyPutFailure,

		countKeyEjectionDueToFullCapacity: countKeyEjectionDueToFullCapacity,
		countKeyEjectionDueToEmergency:    countKeyEjectionDueToEmergency,
	}
}

// BucketAvailableSlots keeps track of number of available slots in buckets of cache.
func (h *HeroCacheCollector) BucketAvailableSlots(availableSlots uint64, totalSlots uint64) {
	normalizedAvailableSlots := float64(availableSlots) / float64(totalSlots)
	h.histogramNormalizedBucketSlotAvailable.Observe(normalizedAvailableSlots)
}

// OnKeyPutSuccess is called whenever a new (key, entity) pair is successfully added to the cache.
func (h *HeroCacheCollector) OnKeyPutSuccess() {
	h.countKeyPutSuccess.Inc()
}

// OnKeyPutFailure is tracking the total number of unsuccessful writes caused by adding a duplicate key to the cache.
// A duplicate key is dropped by the cache when it is written to the cache.
// Note: in context of HeroCache, the key corresponds to the identifier of its entity. Hence, a duplicate key corresponds to
// a duplicate entity.
func (h *HeroCacheCollector) OnKeyPutFailure() {
	h.countKeyPutFailure.Inc()
}

// OnKeyGetSuccess tracks total number of successful read queries.
// A read query is successful if the entity corresponding to its key is available in the cache.
// Note: in context of HeroCache, the key corresponds to the identifier of its entity.
func (h *HeroCacheCollector) OnKeyGetSuccess() {
	h.countKeyGetSuccess.Inc()
}

// OnKeyGetFailure tracks total number of unsuccessful read queries.
// A read query is unsuccessful if the entity corresponding to its key is not available in the cache.
// Note: in context of HeroCache, the key corresponds to the identifier of its entity.
func (h *HeroCacheCollector) OnKeyGetFailure() {
	h.countKeyGetFailure.Inc()
}

// OnEntityEjectionDueToFullCapacity is called whenever adding a new (key, entity) to the cache results in ejection of another (key', entity') pair.
// This normally happens when the cache is full.
// Note: in context of HeroCache, the key corresponds to the identifier of its entity.
func (h *HeroCacheCollector) OnEntityEjectionDueToFullCapacity() {
	h.countKeyEjectionDueToFullCapacity.Inc()
}

// OnEntityEjectionDueToEmergency is called whenever a bucket is found full and all of its keys are valid, i.e.,
// each key belongs to an existing (key, entity) pair.
// Hence, adding a new key to that bucket will replace the oldest valid key inside that bucket.
// Note: in context of HeroCache, the key corresponds to the identifier of its entity.
func (h *HeroCacheCollector) OnEntityEjectionDueToEmergency() {
	h.countKeyEjectionDueToEmergency.Inc()
}
