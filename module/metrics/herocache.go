package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/module"
)

const subsystemHeroCache = "hero_cache"

var count = 0

type HeroCacheCollector struct {
	normalizedBucketSlotAvailableHistogram prometheus.Histogram

	successfulReadCount   prometheus.Counter
	unsuccessfulReadCount prometheus.Counter

	successfulWriteCount   prometheus.Counter
	unsuccessfulWriteCount prometheus.Counter

	fullCapacityEntityEjectionCount prometheus.Counter
	emergencyKeyEjectionCount       prometheus.Counter
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

func CollectionNodeTransactionsCacheMetrics(registrar prometheus.Registerer) *HeroCacheCollector {
	if count > 0 {
		panic("duplicate attempt on transaction cache metrics")
	}
	count++
	return NewHeroCacheCollector(namespaceCollection, ResourceTransaction, registrar)
}

func NewHeroCacheCollector(nameSpace string, cacheName string, registrar prometheus.Registerer) *HeroCacheCollector {

	normalizedBucketSlotAvailableHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Buckets:   []float64{0, 0.05, 0.1, 0.25, 0.5, 0.75, 1},
		Name:      cacheName + "_" + "normalized_bucket_available_slot_count",
		Help:      "normalized histogram of available slots across all buckets",
	})

	successfulReadCount := prometheus.NewCounter(prometheus.CounterOpts{
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

	successfulWriteCount := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "successful_write_count_total",
		Help:      "total number successful write queries",
	})

	unsuccessfulWriteCount := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "unsuccessful_write_count_total",
		Help:      "total number of queries writing an already existing (duplicate) entity to the cache",
	})

	fullCapacityEntityEjectionCount := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "full_capacity_entity_ejection_total",
		Help:      "total number of entities ejected when writing new entities at full capacity",
	})

	emergencyKeyEjectionCount := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "emergency_key_ejection_total",
		Help:      "total number of emergency key ejections at bucket level",
	})

	registrar.MustRegister(
		// available slot distribution
		normalizedBucketSlotAvailableHistogram,

		// read
		successfulReadCount,
		unsuccessfulReadCount,

		// write
		successfulWriteCount,
		unsuccessfulWriteCount,

		// ejection
		fullCapacityEntityEjectionCount,
		emergencyKeyEjectionCount)

	return &HeroCacheCollector{
		normalizedBucketSlotAvailableHistogram: normalizedBucketSlotAvailableHistogram,

		successfulReadCount:   unsuccessfulReadCount,
		unsuccessfulReadCount: unsuccessfulReadCount,

		successfulWriteCount:   successfulWriteCount,
		unsuccessfulWriteCount: unsuccessfulWriteCount,

		fullCapacityEntityEjectionCount: fullCapacityEntityEjectionCount,
		emergencyKeyEjectionCount:       emergencyKeyEjectionCount,
	}
}

// BucketAvailableSlots keeps track of number of available slots in buckets of cache.
func (h *HeroCacheCollector) BucketAvailableSlots(availableSlots uint64, totalSlots uint64) {
	normalizedAvailableSlots := float64(availableSlots) / float64(totalSlots)
	h.normalizedBucketSlotAvailableHistogram.Observe(normalizedAvailableSlots)
}

// OnSuccessfulWrite is called whenever a new entity is successfully added to the cache.
func (h *HeroCacheCollector) OnSuccessfulWrite() {
	h.successfulWriteCount.Inc()
}

// OnUnsuccessfulWrite is tracking the total number of unsuccessful writes caused by adding a duplicate entity to the cache.
// A duplicate entity is dropped by the cache when it is written to the cache.
func (h *HeroCacheCollector) OnUnsuccessfulWrite() {
	h.unsuccessfulWriteCount.Inc()
}

// OnEntityEjectedAtFullCapacity is called whenever adding a new entity to the cache results in ejection of another entity.
// This normally happens when the cache is full.
func (h *HeroCacheCollector) OnEntityEjectedAtFullCapacity() {
	h.fullCapacityEntityEjectionCount.Inc()
}

// OnEmergencyKeyEjection is called whenever a bucket is found full and all of its keys are valid.
// Hence, adding a new entity to that bucket will replace the oldest valid key inside that bucket.
func (h *HeroCacheCollector) OnEmergencyKeyEjection() {
	h.emergencyKeyEjectionCount.Inc()
}

// OnSuccessfulRead tracks total number of successful read queries. A read query is successful if its entity is available in the cache.
func (h *HeroCacheCollector) OnSuccessfulRead() {
	h.successfulReadCount.Inc()
}

// OnUnsuccessfulRead tracks total number of unsuccessful read queries. A read query is unsuccessful if its entity is not available in the cache.
func (h *HeroCacheCollector) OnUnsuccessfulRead() {
	h.unsuccessfulReadCount.Inc()
}
