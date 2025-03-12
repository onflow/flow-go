package herocache

import (
	"encoding/binary"
	"github.com/onflow/flow-go/model/flow"
	"time"
	_ "unsafe" // for linking runtimeNano

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/utils/logging"
)

//go:linkname runtimeNano runtime.nanotime
func runtimeNano() int64

const (
	slotsPerBucket = uint64(16)

	// slotAgeUnallocated defines an unallocated slot with zero age.
	slotAgeUnallocated = uint64(0)

	// telemetryCounterInterval is the number of required interactions with
	// this back data prior to printing any log. This is done as a slow-down mechanism
	// to avoid spamming logs upon read/write heavy operations. An interaction can be
	// a read or write.
	telemetryCounterInterval = uint64(10_000)

	// telemetryDurationInterval is the required elapsed duration interval
	// prior to printing any log. This is done as a slow-down mechanism
	// to avoid spamming logs upon read/write heavy operations.
	telemetryDurationInterval = 10 * time.Second
)

// bucketIndex is data type representing a bucket index.
type bucketIndex uint64

// slotIndex is data type representing a slot index in a bucket.
type slotIndex uint64

// sha32of256 is a 32-bits prefix flow.Identifier used to determine the bucketIndex of the value
// it represents.
type sha32of256 uint32

// slot is an internal notion corresponding to the identifier of an value that is
// meant to be stored in this Cache.
type slot struct {
	slotAge        uint64          // age of this slot.
	valueIndex     heropool.EIndex // link to actual value.
	valueId32of256 sha32of256      // the 32-bits prefix of value identifier.
}

// slotBucket represents a bucket of slots.
type slotBucket struct {
	slots [slotsPerBucket]slot
}

// Cache implements an array-based generic memory pool backed by a fixed total array.
// Note that this implementation is NOT thread-safe, and the higher-level Backend is responsible for concurrency management.
type Cache[V any] struct {
	logger    zerolog.Logger
	collector module.HeroCacheMetrics
	// NOTE: as a BackData implementation, Cache must be non-blocking.
	// Concurrency management is done by overlay Backend.
	sizeLimit    uint32
	slotCount    uint64 // total number of non-expired key-values
	bucketNum    uint64 // total number of buckets (i.e., total of buckets)
	ejectionMode heropool.EjectionMode
	// buckets keeps the slots (i.e., valueId) of the (key, value) pairs that are maintained in this BackData.
	buckets []slotBucket
	// entities keeps the values (i.e., entity) of the (key, value) pairs that are maintained in this BackData.
	entities *heropool.Pool[flow.Identifier, V]
	// telemetry
	//
	// availableSlotHistogram[i] represents number of buckets with i
	// available (i.e., empty) slots to take.
	availableSlotHistogram []uint64
	// interactionCounter keeps track of interactions made with
	// Cache. Invoking any methods of this BackData is considered
	// towards an interaction. The interaction counter is set to zero whenever
	// it reaches a predefined limit. Its purpose is to manage the speed at which
	// telemetry logs are printed.
	interactionCounter *atomic.Uint64
	// lastTelemetryDump keeps track of the last time telemetry logs dumped.
	// Its purpose is to manage the speed at which telemetry logs are printed.
	lastTelemetryDump *atomic.Int64
	// tracer reports ejection events, initially nil but can be injection using CacheOpt
	tracer Tracer[V]
}

// DefaultOversizeFactor determines the default oversizing factor of HeroCache.
// What is oversize factor?
// Imagine adding n keys, rounds times to a hash table with a fixed number slots per bucket.
// The number of buckets can be chosen upon initialization and then never changes.
// If a bucket is full then the oldest key is ejected, and if that key is too new, this is a bucket overflow.
// How many buckets are needed to avoid a bucket overflow assuming cryptographic key hashing is used?
// The overSizeFactor is used to determine the number of buckets.
// Assume n 16, rounds 3, & slotsPerBucket 3 for the tiny example below showing overSizeFactor 1 thru 6.
// As overSizeFactor is increased the chance of overflowing a bucket is decreased.
// With overSizeFactor 1:  8 from 48 keys can be added before bucket overflow.
// With overSizeFactor 2:  10 from 48 keys can be added before bucket overflow.
// With overSizeFactor 3:  13 from 48 keys can be added before bucket overflow.
// With overSizeFactor 4:  15 from 48 keys can be added before bucket overflow.
// With overSizeFactor 5:  27 from 48 keys can be added before bucket overflow.
// With overSizeFactor 6:  48 from 48 keys can be added.
// The default overSizeFactor factor is different in the package code because slotsPerBucket is > 3.
const DefaultOversizeFactor = uint32(8)

func NewCache[V any](
	sizeLimit uint32,
	oversizeFactor uint32,
	ejectionMode heropool.EjectionMode,
	logger zerolog.Logger,
	collector module.HeroCacheMetrics,
	opts ...CacheOpt[V],
) *Cache[V] {

	// total buckets.
	capacity := uint64(sizeLimit * oversizeFactor)
	bucketNum := capacity / slotsPerBucket
	if bucketNum == 0 {
		// we panic here because we don't want to continue with a zero bucketNum (it can cause a DoS attack).
		panic("bucketNum cannot be zero, choose a bigger sizeLimit or a smaller oversizeFactor")
	}

	if capacity%slotsPerBucket != 0 {
		// accounting for remainder.
		bucketNum++
	}

	bd := &Cache[V]{
		logger:                 logger,
		collector:              collector,
		bucketNum:              bucketNum,
		sizeLimit:              sizeLimit,
		buckets:                make([]slotBucket, bucketNum),
		ejectionMode:           ejectionMode,
		entities:               heropool.NewHeroPool[flow.Identifier, V](sizeLimit, ejectionMode, logger),
		availableSlotHistogram: make([]uint64, slotsPerBucket+1), // +1 is to account for empty buckets as well.
		interactionCounter:     atomic.NewUint64(0),
		lastTelemetryDump:      atomic.NewInt64(0),
	}

	// apply extra options
	for _, opt := range opts {
		opt(bd)
	}

	return bd
}

// Has checks if backdata already contains the value with the given identifier.
func (c *Cache[V]) Has(key flow.Identifier) bool {
	defer c.logTelemetry()

	_, _, _, ok := c.get(key)
	return ok
}

// Add adds the given value to the backdata and returns true if the value was added or false if
// a valid value already exists for the provided ID.
func (c *Cache[V]) Add(key flow.Identifier, value V) bool {
	defer c.logTelemetry()
	return c.put(key, value)
}

// Remove removes the value with the given identifier and returns the removed value and true if
// the value was removed or false if the value was not found.
func (c *Cache[V]) Remove(key flow.Identifier) (value V, ok bool) {
	defer c.logTelemetry()

	value, bucketIndex, sliceIndex, exists := c.get(key)
	if !exists {
		return value, false
	}
	// removes value from underlying entities list.
	c.invalidateValue(bucketIndex, sliceIndex)

	// frees up slot
	c.unuseSlot(bucketIndex, sliceIndex)

	c.collector.OnKeyRemoved(c.entities.Size())
	return value, true
}

// Adjust adjusts the value using the given function if the given identifier can be found.
// Returns a bool which indicates whether the value was updated as well as the updated value.
func (c *Cache[V]) Adjust(key flow.Identifier, f func(V) V) (value V, ok bool) {
	defer c.logTelemetry()

	value, removed := c.Remove(key)
	if !removed {
		return value, false
	}

	newValue := f(value)

	// TODO(malleability, #7074): Think of a better solution cause of removing and inserting value with same ID https://github.com/onflow/flow-go/issues/7074
	c.put(key, newValue)

	return newValue, true
}

// AdjustWithInit adjusts the value using the given function if the given identifier can be found. When the
// value is not found, it initializes the value using the given init function and then applies the adjust function.
// Args:
// - key: the identifier of the value to adjust.
// - adjust: the function that adjusts the value.
// - init: the function that initializes the value when it is not found.
// Returns:
//   - the adjusted value.
//
// - a bool which indicates whether the value was adjusted.
func (c *Cache[V]) AdjustWithInit(key flow.Identifier, adjust func(V) V, init func() V) (V, bool) {
	defer c.logTelemetry()

	if c.Has(key) {
		return c.Adjust(key, adjust)
	}
	c.put(key, init())
	return c.Adjust(key, adjust)
}

// Get returns the given value from the backdata.
func (c *Cache[V]) Get(key flow.Identifier) (V, bool) {
	defer c.logTelemetry()

	value, _, _, ok := c.get(key)
	return value, ok
}

// Size returns the size of the backdata, i.e., total number of stored (key, value) pairs.
func (c *Cache[V]) Size() uint {
	defer c.logTelemetry()

	return uint(c.entities.Size())
}

// Head returns the head of queue.
// Boolean return value determines whether there is a head available.
func (c *Cache[V]) Head() (V, bool) {
	return c.entities.Head()
}

// All returns all entities stored in the backdata.
func (c *Cache[V]) All() map[flow.Identifier]V {
	defer c.logTelemetry()

	entitiesList := c.entities.All()
	all := make(map[flow.Identifier]V, len(c.entities.All()))

	total := len(entitiesList)
	for i := 0; i < total; i++ {
		p := entitiesList[i]
		all[p.Id()] = p.Entity()
	}

	return all
}

// Keys returns the list of identifiers of entities stored in the backdata.
func (c *Cache[V]) Keys() flow.IdentifierList {
	defer c.logTelemetry()

	ids := make(flow.IdentifierList, c.entities.Size())
	for i, p := range c.entities.All() {
		ids[i] = p.Id()
	}

	return ids
}

// Values returns the list of entities stored in the backdata.
func (c *Cache[V]) Values() []V {
	defer c.logTelemetry()

	entities := make([]V, c.entities.Size())
	for i, p := range c.entities.All() {
		entities[i] = p.Entity()
	}

	return entities
}

// Clear removes all entities from the backdata.
func (c *Cache[V]) Clear() {
	defer c.logTelemetry()

	c.buckets = make([]slotBucket, c.bucketNum)
	c.entities = heropool.NewHeroPool[flow.Identifier, V](c.sizeLimit, c.ejectionMode, c.logger)
	c.availableSlotHistogram = make([]uint64, slotsPerBucket+1)
	c.interactionCounter = atomic.NewUint64(0)
	c.lastTelemetryDump = atomic.NewInt64(0)
	c.slotCount = 0
}

// put writes the (key, value) pair into this BackData. Boolean return value
// determines whether the write operation was successful. A write operation fails when there is already
// a duplicate entityId exists in the BackData, and that entityId is linked to a valid entity.
func (c *Cache[V]) put(key flow.Identifier, value V) bool {
	c.collector.OnKeyPutAttempt(c.entities.Size())

	entityId32of256, b := c.entityId32of256AndBucketIndex(key)
	slotToUse, unique := c.slotIndexInBucket(b, entityId32of256, key)
	if !unique {
		// entityId already exists
		c.collector.OnKeyPutDeduplicated()
		return false
	}

	if linkedId, _, ok := c.linkedValueOf(b, slotToUse); ok {
		// bucket is full, and we are replacing an already linked (but old) slot that has a valid value, hence
		// we should remove its value from underlying entities list.
		ejectedEntity := c.invalidateValue(b, slotToUse)
		if c.tracer != nil {
			c.tracer.EntityEjectionDueToEmergency(ejectedEntity)
		}
		c.collector.OnEntityEjectionDueToEmergency()
		c.logger.Warn().
			Hex("replaced_entity_id", logging.ID(linkedId)).
			Hex("added_entity_id", logging.ID(key)).
			Msg("emergency ejection, adding entity to cache resulted in replacing a valid key, potential collision")
	}

	c.slotCount++
	entityIndex, slotAvailable, ejectedEntity, wasEjected := c.entities.Add(key, value, c.ownerIndexOf(b, slotToUse))
	if !slotAvailable {
		c.collector.OnKeyPutDrop()
		return false
	}

	if wasEjected {
		// cache is at its full size and ejection happened to make room for this new entity.
		if c.tracer != nil {
			c.tracer.EntityEjectionDueToFullCapacity(ejectedEntity)
		}
		c.collector.OnEntityEjectionDueToFullCapacity()
	}

	c.buckets[b].slots[slotToUse].slotAge = c.slotCount
	c.buckets[b].slots[slotToUse].valueIndex = entityIndex
	c.buckets[b].slots[slotToUse].valueId32of256 = entityId32of256
	c.collector.OnKeyPutSuccess(c.entities.Size())
	return true
}

// get retrieves the value corresponding to given identifier from underlying entities list.
// The boolean return value determines whether an value with given id exists in the BackData.
func (c *Cache[V]) get(key flow.Identifier) (value V, bckIndex bucketIndex, sltIndex slotIndex, ok bool) {
	entityId32of256, b := c.entityId32of256AndBucketIndex(key)
	for s := slotIndex(0); s < slotIndex(slotsPerBucket); s++ {
		if c.buckets[b].slots[s].valueId32of256 != entityId32of256 {
			continue
		}

		id, linkedValue, linked := c.linkedValueOf(b, s)
		if !linked {
			// no linked entity for this (bucketIndex, slotIndex) pair.
			c.collector.OnKeyGetFailure()
			return value, 0, 0, false
		}

		if id != key {
			// checking identifiers fully.
			continue
		}

		c.collector.OnKeyGetSuccess()
		return linkedValue, b, s, true
	}

	c.collector.OnKeyGetFailure()
	return value, 0, 0, false
}

// entityId32of256AndBucketIndex determines the id prefix as well as the bucket index corresponding to the
// given identifier.
func (c *Cache[V]) entityId32of256AndBucketIndex(id flow.Identifier) (sha32of256, bucketIndex) {
	// uint64(id[0:8]) used to compute bucket index for which this identifier belongs to
	b := binary.LittleEndian.Uint64(id[0:8]) % c.bucketNum

	// uint32(id[8:12]) used to compute a shorter identifier for this id to represent in memory.
	entityId32of256 := binary.LittleEndian.Uint32(id[8:12])

	return sha32of256(entityId32of256), bucketIndex(b)
}

// expiryThreshold returns the threshold for which all slots with index below threshold are considered old enough for eviction.
func (c *Cache[V]) expiryThreshold() uint64 {
	var expiryThreshold uint64 = 0
	if c.slotCount > uint64(c.sizeLimit) {
		// total number of slots written are above the predefined limit
		expiryThreshold = c.slotCount - uint64(c.sizeLimit)
	}

	return expiryThreshold
}

// slotIndexInBucket returns a free slot for this entityId in the bucket. In case the bucket is full, it invalidates the oldest valid slot,
// and returns its index as free slot. It returns false if the entityId already exists in this bucket.
func (c *Cache[V]) slotIndexInBucket(b bucketIndex, slotId sha32of256, entityId flow.Identifier) (slotIndex, bool) {
	slotToUse := slotIndex(0)
	expiryThreshold := c.expiryThreshold()
	availableSlotCount := uint64(0) // for telemetry logs.

	oldestSlotInBucket := c.slotCount + 1 // initializes the oldest slot to current max.

	for s := slotIndex(0); s < slotIndex(slotsPerBucket); s++ {
		if c.buckets[b].slots[s].slotAge < oldestSlotInBucket {
			// record slot s as oldest slot
			oldestSlotInBucket = c.buckets[b].slots[s].slotAge
			slotToUse = s
		}

		if c.buckets[b].slots[s].slotAge <= expiryThreshold {
			// slot technically expired or never assigned
			availableSlotCount++
			continue
		}

		if c.buckets[b].slots[s].valueId32of256 != slotId {
			// slot id is distinct and fresh, and hence move to next slot.
			continue
		}

		id, _, linked := c.linkedValueOf(b, s)
		if !linked {
			// slot is not linked to a valid value, hence, can be used
			// as an available slot.
			availableSlotCount++
			slotToUse = s
			continue
		}

		if id != entityId {
			// slot is fresh, fully distinct, and linked. Hence,
			// moving to next slot.
			continue
		}

		// value ID already exists in the bucket
		return 0, false
	}

	c.availableSlotHistogram[availableSlotCount]++
	c.collector.BucketAvailableSlots(availableSlotCount, slotsPerBucket)
	return slotToUse, true
}

// ownerIndexOf maps the (bucketIndex, slotIndex) pair to a canonical unique (scalar) index.
// This scalar index is used to represent this (bucketIndex, slotIndex) pair in the underlying
// entities list.
func (c *Cache[V]) ownerIndexOf(b bucketIndex, s slotIndex) uint64 {
	return (uint64(b) * slotsPerBucket) + uint64(s)
}

// linkedValueOf returns the value linked to this (bucketIndex, slotIndex) pair from the underlying entities list.
// By a linked value, we mean if the value has an owner index matching to (bucketIndex, slotIndex).
// The bool return value corresponds to whether there is a linked value to this (bucketIndex, slotIndex) or not.
func (c *Cache[V]) linkedValueOf(b bucketIndex, s slotIndex) (key flow.Identifier, value V, ok bool) {
	if c.buckets[b].slots[s].slotAge == slotAgeUnallocated {
		// slotIndex never used, or recently invalidated, hence
		// does not have any linked value
		return flow.Identifier{}, value, false
	}

	// retrieving value index in the underlying values linked-list
	valueIndex := c.buckets[b].slots[s].valueIndex
	id, value, owner := c.entities.Get(valueIndex)
	if c.ownerIndexOf(b, s) != owner {
		// value is not linked to this (bucketIndex, slotIndex)
		c.buckets[b].slots[s].slotAge = slotAgeUnallocated
		return flow.Identifier{}, value, false
	}

	return id, value, true
}

// logTelemetry prints telemetry logs depending on number of interactions and last time telemetry has been logged.
func (c *Cache[V]) logTelemetry() {
	counter := c.interactionCounter.Inc()
	if counter < telemetryCounterInterval {
		// not enough interactions to log.
		return
	}
	if time.Duration(runtimeNano()-c.lastTelemetryDump.Load()) < telemetryDurationInterval {
		// not long elapsed since last log.
		return
	}
	if !c.interactionCounter.CompareAndSwap(counter, 0) {
		// raced on CAS, hence, not logging.
		return
	}

	lg := c.logger.With().
		Uint64("total_slots_written", c.slotCount).
		Uint64("total_interactions_since_last_log", counter).Logger()

	for i := range c.availableSlotHistogram {
		lg = lg.With().
			Int("available_slots", i).
			Uint64("total_buckets", c.availableSlotHistogram[i]).
			Logger()
	}

	lg.Debug().Msg("logging telemetry")
	c.lastTelemetryDump.Store(runtimeNano())
}

// unuseSlot marks slot as free so that it is ready to be re-used.
func (c *Cache[V]) unuseSlot(b bucketIndex, s slotIndex) {
	c.buckets[b].slots[s].slotAge = slotAgeUnallocated
}

// invalidateValue removes the value linked to the specified slot from the underlying entities
// list. So that value slot is made available to take if needed.
func (c *Cache[V]) invalidateValue(b bucketIndex, s slotIndex) V {
	return c.entities.Remove(c.buckets[b].slots[s].valueIndex)
}
