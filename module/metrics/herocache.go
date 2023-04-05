package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/module"
)

const subsystemHeroCache = "hero_cache"

var _ module.HeroCacheMetrics = (*HeroCacheCollector)(nil)

type HeroCacheCollector struct {
	histogramNormalizedBucketSlotAvailable prometheus.Histogram

	countKeyGetSuccess prometheus.Counter
	countKeyGetFailure prometheus.Counter

	countKeyPutSuccess      prometheus.Counter
	countKeyPutDrop         prometheus.Counter
	countKeyPutDeduplicated prometheus.Counter
	countKeyPutAttempt      prometheus.Counter
	countKeyRemoved         prometheus.Counter

	size prometheus.Gauge

	countKeyEjectionDueToFullCapacity prometheus.Counter
	countKeyEjectionDueToEmergency    prometheus.Counter
}

type HeroCacheMetricsRegistrationFunc func(uint64) module.HeroCacheMetrics

func NetworkReceiveCacheMetricsFactory(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingReceiveCache, registrar)
}

func PublicNetworkReceiveCacheMetricsFactory(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceNetwork, ResourcePublicNetworkingReceiveCache, registrar)
}

func NetworkDnsTxtCacheMetricsFactory(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingDnsTxtCache, registrar)
}

func NetworkDnsIpCacheMetricsFactory(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingDnsIpCache, registrar)
}

func ChunkDataPackRequestQueueMetricsFactory(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceExecution, ResourceChunkDataPackRequests, registrar)
}

func ReceiptRequestsQueueMetricFactory(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceExecution, ResourceReceipt, registrar)
}

func CollectionRequestsQueueMetricFactory(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceCollection, ResourceCollection, registrar)
}

func DisallowListNotificationQueueMetricFactory(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingDisallowListNotificationQueue, registrar)
}

func GossipSubRPCValidationInspectorQueueMetricFactory(publicNetwork bool, registrar prometheus.Registerer) *HeroCacheCollector {
	if publicNetwork {
		return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingPublicRpcValidationInspectorQueue, registrar)
	}
	return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingRpcValidationInspectorQueue, registrar)
}

func GossipSubRPCMetricsObserverInspectorQueueMetricFactory(publicNetwork bool, registrar prometheus.Registerer) *HeroCacheCollector {
	if publicNetwork {
		return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingPublicRpcMetricsObserverInspectorQueue, registrar)
	}
	return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingRpcMetricsObserverInspectorQueue, registrar)
}

func RpcInspectorNotificationQueueMetricFactory(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceNetwork, ResourceNetworkingRpcInspectorNotificationQueue, registrar)
}

func CollectionNodeTransactionsCacheMetrics(registrar prometheus.Registerer, epoch uint64) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceCollection, fmt.Sprintf("%s_%d", ResourceTransaction, epoch), registrar)
}

func FollowerCacheMetrics(registrar prometheus.Registerer) *HeroCacheCollector {
	return NewHeroCacheCollector(namespaceFollowerEngine, ResourceFollowerPendingBlocksCache, registrar)
}

func NewHeroCacheCollector(nameSpace string, cacheName string, registrar prometheus.Registerer) *HeroCacheCollector {

	histogramNormalizedBucketSlotAvailable := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,

		// Note that the notion of "bucket" in HeroCache differs from Prometheus.
		// A HeroCache "bucket" is used to group the keys of the entities.
		// A Prometheus "bucket" is used to group collected data points within a range.
		// This metric represents the histogram of normalized available slots in buckets, where
		// a data point set to 1 represents a bucket with all slots available (i.e., a fully empty bucket),
		// and a data point set to 0 means a bucket with no available slots (i.e., a completely full bucket).
		//
		// We generally set total slots of a bucket in HeroCache to 16. Hence:
		// Prometheus bucket 1 represents total number of HeroCache buckets with at most 16 available slots.
		// Prometheus bucket 0.75 represents total number of HeroCache buckets with at most 12 available slots.
		// Prometheus bucket 0.5 represents total number of HeroCache buckets with at most 8 available slots.
		// Prometheus bucket 0.25 represents total number of HeroCache buckets with at most 4 available slots.
		// Prometheus bucket 0.1 represents total number of HeroCache buckets with at most 1 available slots.
		// Prometheus bucket 0 represents total number of HeroCache buckets with no (i.e., zero) available slots.
		Buckets: []float64{0, 0.1, 0.25, 0.5, 0.75, 1},
		Name:    cacheName + "_" + "normalized_bucket_available_slot_count",
		Help:    "normalized histogram of available slots across all buckets",
	})

	size := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "items_total",
		Help:      "total number of items in the cache",
	})

	countKeyGetSuccess := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "successful_read_count_total",
		Help:      "total number of successful read queries",
	})

	countKeyGetFailure := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "unsuccessful_read_count_total",
		Help:      "total number of unsuccessful read queries",
	})

	countKeyPutAttempt := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "write_attempt_count_total",
		Help:      "total number of put queries",
	})

	countKeyPutDrop := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "write_drop_count_total",
		Help:      "total number of put queries dropped due to full capacity",
	})

	countKeyPutSuccess := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "successful_write_count_total",
		Help:      "total number successful write queries",
	})

	countKeyPutDeduplicated := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "unsuccessful_write_count_total",
		Help:      "total number of queries writing an already existing (duplicate) entity to the cache",
	})

	countKeyRemoved := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: nameSpace,
		Subsystem: subsystemHeroCache,
		Name:      cacheName + "_" + "removed_count_total",
		Help:      "total number of entities removed from the cache",
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

		// size
		size,

		// read
		countKeyGetSuccess,
		countKeyGetFailure,

		// write
		countKeyPutSuccess,
		countKeyPutDeduplicated,
		countKeyPutDrop,
		countKeyPutAttempt,

		// remove
		countKeyRemoved,

		// ejection
		countKeyEjectionDueToFullCapacity,
		countKeyEjectionDueToEmergency)

	return &HeroCacheCollector{
		histogramNormalizedBucketSlotAvailable: histogramNormalizedBucketSlotAvailable,
		size:                                   size,
		countKeyGetSuccess:                     countKeyGetSuccess,
		countKeyGetFailure:                     countKeyGetFailure,

		countKeyPutAttempt:      countKeyPutAttempt,
		countKeyPutSuccess:      countKeyPutSuccess,
		countKeyPutDeduplicated: countKeyPutDeduplicated,
		countKeyPutDrop:         countKeyPutDrop,

		countKeyRemoved: countKeyRemoved,

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
// size parameter is the current size of the cache post insertion.
func (h *HeroCacheCollector) OnKeyPutSuccess(size uint32) {
	h.countKeyPutSuccess.Inc()
	h.size.Set(float64(size))
}

// OnKeyPutDeduplicated is tracking the total number of unsuccessful writes caused by adding a duplicate key to the cache.
// A duplicate key is dropped by the cache when it is written to the cache.
// Note: in context of HeroCache, the key corresponds to the identifier of its entity. Hence, a duplicate key corresponds to
// a duplicate entity.
func (h *HeroCacheCollector) OnKeyPutDeduplicated() {
	h.countKeyPutDeduplicated.Inc()
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

// OnKeyPutAttempt is called whenever a new (key, value) pair is attempted to be put in cache.
// It does not reflect whether the put was successful or not.
// A (key, value) pair put attempt may fail if the cache is full, or the key already exists.
// size parameter is the current size of the cache prior to the put attempt.
func (h *HeroCacheCollector) OnKeyPutAttempt(size uint32) {
	h.countKeyPutAttempt.Inc()
	h.size.Set(float64(size))
}

// OnKeyPutDrop is called whenever a new (key, entity) pair is dropped from the cache due to full cache.
func (h *HeroCacheCollector) OnKeyPutDrop() {
	h.countKeyPutDrop.Inc()
}

// OnKeyRemoved is called whenever a (key, entity) pair is removed from the cache.
// size parameter is the current size of the cache.
func (h *HeroCacheCollector) OnKeyRemoved(size uint32) {
	h.countKeyRemoved.Inc()
	h.size.Set(float64(size))
}

// OnEntityEjectionDueToFullCapacity is called whenever adding a new (key, entity) to the cache results in ejection of another (key', entity') pair.
// This normally happens -- and is expected -- when the cache is full.
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
