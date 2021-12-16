package backdata

import (
	"encoding/binary"
	"time"
	_ "unsafe" // for linking runtimeNano

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/stdmap/backdata/arraylinkedlist"
)

//go:linkname runtimeNano runtime.nanotime
func runtimeNano() int64

const (
	bucketSize = uint64(16)

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

// idPrefix is a 32-bits prefix flow.Identifier used to determine the bIndex of the entity
// it represents.
type idPrefix uint32

type key struct {
	keyIndex    uint64                 // slot age.
	entityIndex arraylinkedlist.EIndex // link to actual entity.
	idPref      idPrefix               // 32-bits prefix of entity identifier.
}

// keyBucket represents a bucket of keys.
type keyBucket [bucketSize]key

// ArrayBackData implements an array-based generic memory pool backed by a fixed total array.
type ArrayBackData struct {
	logger zerolog.Logger
	// NOTE: as a BackData implementation, ArrayBackData must be non-blocking.
	// Concurrency management is done by overlay Backend.
	limit        uint32
	keyCount     uint64 // total number of non-expired key-values
	bucketNum    uint64 // total number of buckets (i.e., total of buckets)
	ejectionMode arraylinkedlist.EjectionMode
	// buckets keeps the keys (i.e., entityId) of the (entityId, entity) pairs that are maintained in this BackData.
	buckets []keyBucket
	// entities keeps the values (i.e., entity) of the (entityId, entity) pairs that are maintained in this BackData.
	entities *arraylinkedlist.EntityDoubleLinkedList

	// telemetry
	//
	// availableSlotHistogram[i] represents number of buckets with i
	// available (i.e., empty) slots to take.
	availableSlotHistogram []uint64
	// interactionCounter keeps track of interactions made with
	// ArrayBackData. Invoking any methods of this BackData is considered
	// towards an interaction. The interaction counter is set to zero whenever
	// it reaches a predefined limit. Its purpose is to manage the speed at which
	// telemetry logs are printed.
	interactionCounter uint64
	// lastTelemetryDump keeps track of the last time telemetry logs dumped.
	// Its purpose is to manage the speed at which telemetry logs are printed.
	lastTelemetryDump int64
}

func NewArrayBackData(limit uint32, oversizeFactor uint32, ejectionMode arraylinkedlist.EjectionMode, logger zerolog.Logger) *ArrayBackData {
	// total buckets.
	capacity := uint64(limit * oversizeFactor)
	bucketNum := capacity / bucketSize
	if capacity%bucketSize != 0 {
		// accounting for remainder.
		bucketNum++
	}

	bd := &ArrayBackData{
		logger:                 logger,
		bucketNum:              bucketNum,
		limit:                  limit,
		buckets:                make([]keyBucket, bucketNum),
		ejectionMode:           ejectionMode,
		entities:               arraylinkedlist.NewEntityList(limit, ejectionMode),
		availableSlotHistogram: make([]uint64, bucketSize+1), // +1 is to account for empty buckets as well.
	}

	return bd
}

// Has checks if we already contain the entity with the given identifier.
func (a *ArrayBackData) Has(entityID flow.Identifier) bool {
	defer a.logTelemetry()

	_, _, _, ok := a.get(entityID)
	return ok
}

// Add adds the given item to the BackData.
func (a *ArrayBackData) Add(entityID flow.Identifier, entity flow.Entity) bool {
	defer a.logTelemetry()

	return a.put(entityID, entity)
}

// Rem will remove the item with the given identity.
func (a *ArrayBackData) Rem(entityID flow.Identifier) (flow.Entity, bool) {
	defer a.logTelemetry()

	entity, bucketIndex, sliceIndex, exists := a.get(entityID)
	if !exists {
		return nil, false
	}
	// removes value from underlying entities list.
	a.invalidateEntity(bucketIndex, sliceIndex)

	// frees up key
	a.invalidateKey(bucketIndex, sliceIndex)

	return entity, true
}

// Adjust will adjust the value item using the given function if the given key can be found.
// Returns a bool which indicates whether the value was updated as well as the updated value.
func (a *ArrayBackData) Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool) {
	defer a.logTelemetry()

	entity, removed := a.Rem(entityID)
	if !removed {
		return nil, false
	}

	newEntity := f(entity)
	newEntityID := newEntity.ID()

	a.put(newEntityID, newEntity)

	return newEntity, true
}

// ByID returns the given entity from the BackData.
func (a *ArrayBackData) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	defer a.logTelemetry()

	entity, _, _, ok := a.get(entityID)
	return entity, ok
}

// Size will return the total size of BackData, i.e., total number of valid (entityId, entity) pairs.
func (a ArrayBackData) Size() uint {
	defer a.logTelemetry()

	return uint(a.entities.Size())
}

// All returns all entities from the BackData.
func (a ArrayBackData) All() map[flow.Identifier]flow.Entity {
	defer a.logTelemetry()

	all := make(map[flow.Identifier]flow.Entity)
	for bucketIndex, bucket := range a.buckets {
		for slotIndex := range bucket {
			id, entity, linked := a.linkedEntityOf(bIndex(bucketIndex), sIndex(slotIndex))
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
func (a *ArrayBackData) Clear() {
	defer a.logTelemetry()

	a.buckets = make([]keyBucket, a.bucketNum)
	a.entities = arraylinkedlist.NewEntityList(a.limit, a.ejectionMode)
	a.availableSlotHistogram = make([]uint64, bucketSize+1)
	a.interactionCounter = 0
	a.lastTelemetryDump = 0
	a.keyCount = 0
}

// Hash will use a merkle root hash to hash all items.
func (a *ArrayBackData) Hash() flow.Identifier {
	defer a.logTelemetry()

	return flow.MerkleRoot(flow.GetIDs(a.All())...)
}

// put writes the (entityId, entity) pair into this BackData. Boolean return value
// determines whether the write operation was successful. A write operation fails when there is already
// a duplicate entityId exists in the BackData, and that entityId is linked to a valid entity.
func (a *ArrayBackData) put(entityId flow.Identifier, entity flow.Entity) bool {
	idPref, bucketIndex := a.idPrefixAndBucketIndex(entityId)
	slotToUse, unique := a.slotInBucket(bucketIndex, idPref, entityId)
	if !unique {
		// entityId already exists
		return false
	}

	if _, _, ok := a.linkedEntityOf(bucketIndex, slotToUse); ok {
		// we are replacing an already linked (but old) key that has a valid value, hence
		// we should remove its value from underlying entities list.
		a.invalidateEntity(bucketIndex, slotToUse)
	}

	a.keyCount++
	entityIndex := a.entities.Add(entityId, entity, a.ownerIndexOf(bucketIndex, slotToUse))
	a.buckets[bucketIndex][slotToUse].keyIndex = a.keyCount
	a.buckets[bucketIndex][slotToUse].entityIndex = entityIndex
	a.buckets[bucketIndex][slotToUse].idPref = idPref
	return true
}

// get retrieves the entity corresponding to given identifier from underlying entities list.
// The boolean return value determines whether an entity with given id exists in the BackData.
func (a *ArrayBackData) get(entityID flow.Identifier) (flow.Entity, bIndex, sIndex, bool) {
	idPref, bucketIndex := a.idPrefixAndBucketIndex(entityID)
	for slotIndex := sIndex(0); slotIndex < sIndex(bucketSize); slotIndex++ {
		if a.buckets[bucketIndex][slotIndex].idPref != idPref {
			continue
		}

		id, entity, linked := a.linkedEntityOf(bucketIndex, slotIndex)
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
func (a ArrayBackData) idPrefixAndBucketIndex(id flow.Identifier) (idPrefix, bIndex) {
	// uint64(id[0:8]) used to compute bucket index for which this key (i.e., id) belongs to
	bucketIndex := binary.LittleEndian.Uint64(id[0:8]) % a.bucketNum

	// uint32(id[8:12]) used to compute a shorter key for this id to represent in memory.
	idPref := binary.LittleEndian.Uint32(id[8:12])

	return idPrefix(idPref), bIndex(bucketIndex)
}

// expiryThreshold returns the threshold for which all keys with index below threshold are considered old enough for eviction.
func (a ArrayBackData) expiryThreshold() uint64 {
	var expiryThreshold uint64 = 0
	if a.keyCount > uint64(a.limit) {
		// total number of keys written are above the predefined limit
		expiryThreshold = a.keyCount - uint64(a.limit)
	}

	return expiryThreshold
}

// slotInBucket returns a free slot for this entityId in the bucket. In case the bucket is full, it invalidates the oldest (unlinked) key,
// and returns its index as free slot. It returns false if the entityId already exists in this bucket.
func (a *ArrayBackData) slotInBucket(bucketIndex bIndex, idPref idPrefix, entityId flow.Identifier) (sIndex, bool) {
	slotToUse := sIndex(0)
	expiryThreshold := a.expiryThreshold()
	availableSlotCount := uint64(0) // for telemetry logs.

	oldestKeyInBucket := a.keyCount + 1 // initializes oldest key to current max.

	for s := sIndex(0); s < sIndex(bucketSize); s++ {
		if a.buckets[bucketIndex][s].keyIndex < oldestKeyInBucket {
			// record slot s as oldest slot
			oldestKeyInBucket = a.buckets[bucketIndex][s].keyIndex
			slotToUse = s
		}

		if a.buckets[bucketIndex][s].keyIndex <= expiryThreshold {
			// slot technically expired or never assigned
			availableSlotCount++
			continue
		}

		if a.buckets[bucketIndex][s].idPref != idPref {
			// key is distinct and fresh, and hence move to next key.
			continue
		}

		id, _, linked := a.linkedEntityOf(bucketIndex, s)
		if !linked {
			// key is not linked to a valid entity, hence, can be used
			// as an available slot.
			availableSlotCount++
			slotToUse = s
			continue
		}

		if id != entityId {
			// key is fresh, fully distinct, and linked. Hence,
			// moving to next key.
			continue
		}

		// entity ID already exists in the bucket
		a.availableSlotHistogram[availableSlotCount]++
		return 0, false
	}

	a.availableSlotHistogram[availableSlotCount]++
	return slotToUse, true
}

// ownerIndexOf maps the (bucketIndex, slotIndex) pair to a canonical unique (scalar) index.
// This scalar index is used to represent this (bucketIndex, slotIndex) pair in the underlying
// entities list.
func (a ArrayBackData) ownerIndexOf(bucketIndex bIndex, slotIndex sIndex) uint64 {
	return (uint64(bucketIndex) * bucketSize) + uint64(slotIndex)
}

// linkedEntityOf returns the entity linked to this (bucketIndex, slotIndex) pair from the underlying entities list.
// By a linked entity, we mean if the entity has an owner index matching to (bucketIndex, slotIndex).
// The bool return value corresponds to whether there is a linked entity to this (bucketIndex, slotIndex) or not.
func (a *ArrayBackData) linkedEntityOf(bucketIndex bIndex, slotIndex sIndex) (flow.Identifier, flow.Entity, bool) {
	if a.buckets[bucketIndex][slotIndex].keyIndex == 0 {
		// slotIndex never used, or recently invalidated, hence
		// does not have any linked entity
		return flow.Identifier{}, nil, false
	}

	// retrieving entity index in the underlying entities linked-list
	valueIndex := a.buckets[bucketIndex][slotIndex].entityIndex
	id, entity, owner := a.entities.Get(valueIndex)
	if a.ownerIndexOf(bucketIndex, slotIndex) != owner {
		// entity is not linked to this (bucketIndex, slotIndex)
		a.buckets[bucketIndex][slotIndex].keyIndex = 0
		return flow.Identifier{}, nil, false
	}

	return id, entity, true
}

// logTelemetry prints telemetry logs depending on number of interactions and
// last time telemetry has been logged.
func (a *ArrayBackData) logTelemetry() {
	a.interactionCounter++
	if a.interactionCounter < telemetryCounterInterval {
		// not enough interactions to log.
		return
	}
	if time.Duration(runtimeNano()-a.lastTelemetryDump) < telemetryDurationInterval {
		// not long elapsed since last log.
		return
	}

	lg := a.logger.With().
		Uint64("total_keys_written", a.keyCount).
		Uint64("total_interactions_since_last_log", a.interactionCounter).Logger()

	for i := range a.availableSlotHistogram {
		lg = lg.With().
			Int("available_slots", i).
			Uint64("total_buckets", a.availableSlotHistogram[i]).
			Logger()
	}

	lg.Debug().Msg("logging telemetry")
	a.interactionCounter = 0
	a.lastTelemetryDump = runtimeNano()
}

// invalidateKey sets the key index of specified slot in the bucket to zero, so the key
// is free to take.
func (a *ArrayBackData) invalidateKey(bucketIndex bIndex, slotIndex sIndex) {
	a.buckets[bucketIndex][slotIndex].keyIndex = 0
}

// invalidateEntity removes the entity linked to the specified slot from the underlying entities
// list. So that entity slot is made available to take if needed.
func (a *ArrayBackData) invalidateEntity(bucketIndex bIndex, slotIndex sIndex) {
	a.entities.Rem(a.buckets[bucketIndex][slotIndex].entityIndex)
}
