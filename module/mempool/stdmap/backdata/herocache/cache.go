package herocache

import (
	"encoding/binary"
	"time"
	_ "unsafe" // for linking runtimeNano

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/stdmap/backdata/herocache/pool"
)

//go:linkname runtimeNano runtime.nanotime
func runtimeNano() int64

const (
	slotsPerBucket = uint64(16)

	// telemetryCounterInterval is the number of required interactions with
	// this back data prior to printing any log. This is done as a slow-down mechanism
	// to avoid spamming logs upon read/write heavy operations. An interaction can be
	// a read or write.
	telemetryCounterInterval = uint64(1000)

	// telemetryDurationInterval is the required elapsed duration interval
	// prior to printing any log. This is done as a slow-down mechanism
	// to avoid spamming logs upon read/write heavy operations.
	telemetryDurationInterval = 10 * time.Second
)

// bIndex is data type representing a bucket index.
type bIndex uint64

// sIndex is data type representing a slot index in a bucket.
type sIndex uint64

// sha32of256 is a 32-bits prefix flow.Identifier used to determine the bIndex of the entity
// it represents.
type sha32of256 uint32

// slot is an internal notion corresponding to the identifier of an entity that is
// meant to be stored in this Cache.
type slot struct {
	slotAge     uint64      // age of this slot.
	entityIndex pool.EIndex // link to actual entity.
	slotId      sha32of256  // slot id is the 32-bits prefix of entity identifier.
}

// slotBucket represents a bucket of slots.
type slotBucket [slotsPerBucket]slot

// Cache implements an array-based generic memory pool backed by a fixed total array.
type Cache struct {
	logger zerolog.Logger
	// NOTE: as a BackData implementation, Cache must be non-blocking.
	// Concurrency management is done by overlay Backend.
	limit        uint32
	slotCount    uint64 // total number of non-expired key-values
	bucketNum    uint64 // total number of buckets (i.e., total of buckets)
	ejectionMode pool.EjectionMode
	// buckets keeps the slots (i.e., entityId) of the (entityId, entity) pairs that are maintained in this BackData.
	buckets []slotBucket
	// entities keeps the values (i.e., entity) of the (entityId, entity) pairs that are maintained in this BackData.
	entities *pool.HeroPool

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
	interactionCounter uint64
	// lastTelemetryDump keeps track of the last time telemetry logs dumped.
	// Its purpose is to manage the speed at which telemetry logs are printed.
	lastTelemetryDump int64
}

func NewCache(limit uint32, oversizeFactor uint32, ejectionMode pool.EjectionMode, logger zerolog.Logger) *Cache {
	// total buckets.
	capacity := uint64(limit * oversizeFactor)
	bucketNum := capacity / slotsPerBucket
	if capacity%slotsPerBucket != 0 {
		// accounting for remainder.
		bucketNum++
	}

	bd := &Cache{
		logger:                 logger,
		bucketNum:              bucketNum,
		limit:                  limit,
		buckets:                make([]slotBucket, bucketNum),
		ejectionMode:           ejectionMode,
		entities:               pool.NewPool(limit, ejectionMode),
		availableSlotHistogram: make([]uint64, slotsPerBucket+1), // +1 is to account for empty buckets as well.
	}

	return bd
}

// Has checks if we already contain the entity with the given identifier.
func (c *Cache) Has(entityID flow.Identifier) bool {
	defer c.logTelemetry()

	_, _, _, ok := c.get(entityID)
	return ok
}

// Add adds the given item to the BackData.
func (c *Cache) Add(entityID flow.Identifier, entity flow.Entity) bool {
	defer c.logTelemetry()

	return c.put(entityID, entity)
}

// Rem will remove the item with the given identity.
func (c *Cache) Rem(entityID flow.Identifier) (flow.Entity, bool) {
	defer c.logTelemetry()

	entity, bucketIndex, sliceIndex, exists := c.get(entityID)
	if !exists {
		return nil, false
	}
	// removes value from underlying entities list.
	c.invalidateEntity(bucketIndex, sliceIndex)

	// frees up slot
	c.invalidateSlot(bucketIndex, sliceIndex)

	return entity, true
}

// Adjust will adjust the value item using the given function if the given identifier can be found.
// Returns a bool which indicates whether the value was updated as well as the updated value.
func (c *Cache) Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool) {
	defer c.logTelemetry()

	entity, removed := c.Rem(entityID)
	if !removed {
		return nil, false
	}

	newEntity := f(entity)
	newEntityID := newEntity.ID()

	c.put(newEntityID, newEntity)

	return newEntity, true
}

// ByID returns the given entity from the BackData.
func (c *Cache) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	defer c.logTelemetry()

	entity, _, _, ok := c.get(entityID)
	return entity, ok
}

// Size will return the total size of BackData, i.e., total number of valid (entityId, entity) pairs.
func (c Cache) Size() uint {
	defer c.logTelemetry()

	return uint(c.entities.Size())
}

// All returns all entities from the BackData.
func (c Cache) All() map[flow.Identifier]flow.Entity {
	defer c.logTelemetry()

	all := make(map[flow.Identifier]flow.Entity)
	for bucketIndex, bucket := range c.buckets {
		for slotIndex := range bucket {
			id, entity, linked := c.linkedEntityOf(bIndex(bucketIndex), sIndex(slotIndex))
			if !linked {
				// slot may never be used, or recently invalidated
				continue
			}

			all[id] = entity
		}
	}

	return all
}

// Clear removes all entities from the BackData.
func (c *Cache) Clear() {
	defer c.logTelemetry()

	c.buckets = make([]slotBucket, c.bucketNum)
	c.entities = pool.NewPool(c.limit, c.ejectionMode)
	c.availableSlotHistogram = make([]uint64, slotsPerBucket+1)
	c.interactionCounter = 0
	c.lastTelemetryDump = 0
	c.slotCount = 0
}

// Hash will use a merkle root hash to hash all items.
func (c *Cache) Hash() flow.Identifier {
	defer c.logTelemetry()

	return flow.MerkleRoot(flow.GetIDs(c.All())...)
}

// put writes the (entityId, entity) pair into this BackData. Boolean return value
// determines whether the write operation was successful. A write operation fails when there is already
// a duplicate entityId exists in the BackData, and that entityId is linked to a valid entity.
func (c *Cache) put(entityId flow.Identifier, entity flow.Entity) bool {
	idPref, bucketIndex := c.idPrefixAndBucketIndex(entityId)
	slotToUse, unique := c.slotIndexInBucket(bucketIndex, idPref, entityId)
	if !unique {
		// entityId already exists
		return false
	}

	if _, _, ok := c.linkedEntityOf(bucketIndex, slotToUse); ok {
		// we are replacing an already linked (but old) slot that has a valid value, hence
		// we should remove its value from underlying entities list.
		c.invalidateEntity(bucketIndex, slotToUse)
	}

	c.slotCount++
	entityIndex := c.entities.Add(entityId, entity, c.ownerIndexOf(bucketIndex, slotToUse))
	c.buckets[bucketIndex][slotToUse].slotAge = c.slotCount
	c.buckets[bucketIndex][slotToUse].entityIndex = entityIndex
	c.buckets[bucketIndex][slotToUse].slotId = idPref
	return true
}

// get retrieves the entity corresponding to given identifier from underlying entities list.
// The boolean return value determines whether an entity with given id exists in the BackData.
func (c *Cache) get(entityID flow.Identifier) (flow.Entity, bIndex, sIndex, bool) {
	idPref, bucketIndex := c.idPrefixAndBucketIndex(entityID)
	for slotIndex := sIndex(0); slotIndex < sIndex(slotsPerBucket); slotIndex++ {
		if c.buckets[bucketIndex][slotIndex].slotId != idPref {
			continue
		}

		id, entity, linked := c.linkedEntityOf(bucketIndex, slotIndex)
		if !linked {
			// no linked entity for this (bucketIndex, slotIndex) pair.
			return nil, 0, 0, false
		}

		if id != entityID {
			// checking identifiers fully.
			continue
		}

		return entity, bucketIndex, slotIndex, true
	}

	return nil, 0, 0, false
}

// idPrefixAndBucketIndex determines the id prefix as well as the bucket index corresponding to the
// given identifier.
func (c Cache) idPrefixAndBucketIndex(id flow.Identifier) (sha32of256, bIndex) {
	// uint64(id[0:8]) used to compute bucket index for which this identifier belongs to
	bucketIndex := binary.LittleEndian.Uint64(id[0:8]) % c.bucketNum

	// uint32(id[8:12]) used to compute a shorter identifier for this id to represent in memory.
	idPref := binary.LittleEndian.Uint32(id[8:12])

	return sha32of256(idPref), bIndex(bucketIndex)
}

// expiryThreshold returns the threshold for which all slots with index below threshold are considered old enough for eviction.
func (c Cache) expiryThreshold() uint64 {
	var expiryThreshold uint64 = 0
	if c.slotCount > uint64(c.limit) {
		// total number of slots written are above the predefined limit
		expiryThreshold = c.slotCount - uint64(c.limit)
	}

	return expiryThreshold
}

// slotIndexInBucket returns a free slot for this entityId in the bucket. In case the bucket is full, it invalidates the oldest valid slot,
// and returns its index as free slot. It returns false if the entityId already exists in this bucket.
func (c *Cache) slotIndexInBucket(bucketIndex bIndex, idPref sha32of256, entityId flow.Identifier) (sIndex, bool) {
	slotToUse := sIndex(0)
	expiryThreshold := c.expiryThreshold()
	availableSlotCount := uint64(0) // for telemetry logs.

	oldestSlotInBucket := c.slotCount + 1 // initializes the oldest slot to current max.

	for s := sIndex(0); s < sIndex(slotsPerBucket); s++ {
		if c.buckets[bucketIndex][s].slotAge < oldestSlotInBucket {
			// record slot s as oldest slot
			oldestSlotInBucket = c.buckets[bucketIndex][s].slotAge
			slotToUse = s
		}

		if c.buckets[bucketIndex][s].slotAge <= expiryThreshold {
			// slot technically expired or never assigned
			availableSlotCount++
			continue
		}

		if c.buckets[bucketIndex][s].slotId != idPref {
			// slot id is distinct and fresh, and hence move to next slot.
			continue
		}

		id, _, linked := c.linkedEntityOf(bucketIndex, s)
		if !linked {
			// slot is not linked to a valid entity, hence, can be used
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

		// entity ID already exists in the bucket
		c.availableSlotHistogram[availableSlotCount]++
		return 0, false
	}

	c.availableSlotHistogram[availableSlotCount]++
	return slotToUse, true
}

// ownerIndexOf maps the (bucketIndex, slotIndex) pair to a canonical unique (scalar) index.
// This scalar index is used to represent this (bucketIndex, slotIndex) pair in the underlying
// entities list.
func (c Cache) ownerIndexOf(bucketIndex bIndex, slotIndex sIndex) uint64 {
	return (uint64(bucketIndex) * slotsPerBucket) + uint64(slotIndex)
}

// linkedEntityOf returns the entity linked to this (bucketIndex, slotIndex) pair from the underlying entities list.
// By a linked entity, we mean if the entity has an owner index matching to (bucketIndex, slotIndex).
// The bool return value corresponds to whether there is a linked entity to this (bucketIndex, slotIndex) or not.
func (c *Cache) linkedEntityOf(bucketIndex bIndex, slotIndex sIndex) (flow.Identifier, flow.Entity, bool) {
	if c.buckets[bucketIndex][slotIndex].slotAge == 0 {
		// slotIndex never used, or recently invalidated, hence
		// does not have any linked entity
		return flow.Identifier{}, nil, false
	}

	// retrieving entity index in the underlying entities linked-list
	valueIndex := c.buckets[bucketIndex][slotIndex].entityIndex
	id, entity, owner := c.entities.Get(valueIndex)
	if c.ownerIndexOf(bucketIndex, slotIndex) != owner {
		// entity is not linked to this (bucketIndex, slotIndex)
		c.buckets[bucketIndex][slotIndex].slotAge = 0
		return flow.Identifier{}, nil, false
	}

	return id, entity, true
}

// logTelemetry prints telemetry logs depending on number of interactions and
// last time telemetry has been logged.
func (c *Cache) logTelemetry() {
	c.interactionCounter++
	if c.interactionCounter < telemetryCounterInterval {
		// not enough interactions to log.
		return
	}
	if time.Duration(runtimeNano()-c.lastTelemetryDump) < telemetryDurationInterval {
		// not long elapsed since last log.
		return
	}

	lg := c.logger.With().
		Uint64("total_slots_written", c.slotCount).
		Uint64("total_interactions_since_last_log", c.interactionCounter).Logger()

	for i := range c.availableSlotHistogram {
		lg = lg.With().
			Int("available_slots", i).
			Uint64("total_buckets", c.availableSlotHistogram[i]).
			Logger()
	}

	lg.Debug().Msg("logging telemetry")
	c.interactionCounter = 0
	c.lastTelemetryDump = runtimeNano()
}

// invalidateSlot marks slot as free so that it is ready to be re-used.
func (c *Cache) invalidateSlot(bucketIndex bIndex, slotIndex sIndex) {
	c.buckets[bucketIndex][slotIndex].slotAge = 0
}

// invalidateEntity removes the entity linked to the specified slot from the underlying entities
// list. So that entity slot is made available to take if needed.
func (c *Cache) invalidateEntity(bucketIndex bIndex, slotIndex sIndex) {
	c.entities.Rem(c.buckets[bucketIndex][slotIndex].entityIndex)
}
