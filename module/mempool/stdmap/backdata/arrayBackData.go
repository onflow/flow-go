package backdata

import (
	"encoding/binary"

	zlog "github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/stdmap/backdata/arraylinkedlist"
)

const (
	bucketSize = uint64(16)
)

// bIndex is data type representing a bucket index.
type bIndex uint64

// sIndex is data type representing a slot index in a bucket.
type sIndex uint64

type key struct {
	keyIndex   uint64 // slot age
	valueIndex uint32 // link to key value pair
	idPrefix   uint32 // 32bits of key sha256
}

// keyBucket represents a bucket of keys.
type keyBucket [bucketSize]key

// ArrayBackData implements an array-based generic memory pool backed by a fixed total array.
type ArrayBackData struct {
	// NOTE: as a BackData implementation, ArrayBackData must be non-blocking.
	// Concurrency management is done by overlay Backend.
	limit     uint64
	overLimit uint64
	keyCount  uint64 // total number of non-expired key-values
	bucketNum uint64 // total number of buckets (i.e., total of buckets)
	buckets   []keyBucket
	entities  *arraylinkedlist.EntityDoubleLinkedList

	// temporary data structures for telemetry
	availableSlotHistogram []uint64
}

func NewArrayBackData(limit uint32, oversizeFactor uint32, mode arraylinkedlist.EjectionMode) *ArrayBackData {
	// total buckets
	bucketNum := uint64(limit*oversizeFactor) / bucketSize
	if uint64(limit*oversizeFactor)%bucketSize != 0 {
		bucketNum++
	}

	bd := &ArrayBackData{
		bucketNum:              bucketNum,
		limit:                  uint64(limit),
		overLimit:              uint64(limit * oversizeFactor),
		buckets:                make([]keyBucket, bucketNum),
		entities:               arraylinkedlist.NewEntityList(limit, mode),
		availableSlotHistogram: make([]uint64, bucketSize+1),
	}

	return bd
}

// Has checks if we already contain the item with the given hash.
func (a *ArrayBackData) Has(entityID flow.Identifier) bool {
	_, _, _, ok := a.get(entityID)
	return ok
}

// Add adds the given item to the pool.
func (a *ArrayBackData) Add(entityID flow.Identifier, entity flow.Entity) bool {
	return a.put(entityID, entity)
}

// Rem will remove the item with the given hash.
func (a *ArrayBackData) Rem(entityID flow.Identifier) (flow.Entity, bool) {
	entity, bucketIndex, sliceIndex, exists := a.get(entityID)
	if !exists {
		return nil, false
	}
	// removes value from underlying list
	a.invalidateEntity(bucketIndex, sliceIndex)

	// frees up key
	a.invalidateKey(bucketIndex, sliceIndex)

	return entity, true
}

// Adjust will adjust the value item using the given function if the given key can be found.
// Returns a bool which indicates whether the value was updated as well as the updated value
func (a *ArrayBackData) Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool) {
	entity, removed := a.Rem(entityID)
	if !removed {
		return nil, false
	}

	newEntity := f(entity)
	newEntityID := newEntity.ID()

	a.put(newEntityID, newEntity)

	return newEntity, true
}

// ByID returns the given item from the pool.
func (a *ArrayBackData) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	entity, _, _, ok := a.get(entityID)
	return entity, ok
}

// Size will return the total of the backend.
func (a ArrayBackData) Size() uint {
	return uint(a.entities.Size())
}

// All returns all entities from the pool.
func (a ArrayBackData) All() map[flow.Identifier]flow.Entity {
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

// Clear removes all entities from the pool.
func (a *ArrayBackData) Clear() {

}

// Hash will use a merkle root hash to hash all items.
func (a *ArrayBackData) Hash() flow.Identifier {
	return flow.MerkleRoot(flow.GetIDs(a.All())...)
}

func (a *ArrayBackData) put(entityID flow.Identifier, entity flow.Entity) bool {
	idPrefix, bucketIndex := a.idPrefixAndBucketIndex(entityID)
	slotToUse, unique := a.slotInBucket(bucketIndex, idPrefix, entityID)
	if !unique {
		// entityID already exists
		return false
	}

	if _, _, ok := a.linkedEntityOf(bucketIndex, slotToUse); ok {
		// we are replacing an already linked key with a valid value, hence
		// we should remove its value from underlying entities list.
		a.invalidateEntity(bucketIndex, slotToUse)
	}

	a.keyCount++
	entityIndex := a.entities.Add(entityID, entity, a.ownerIndexOf(bucketIndex, slotToUse))
	a.buckets[bucketIndex][slotToUse].keyIndex = a.keyCount
	a.buckets[bucketIndex][slotToUse].valueIndex = entityIndex
	a.buckets[bucketIndex][slotToUse].idPrefix = idPrefix
	return true
}

func (a *ArrayBackData) get(entityID flow.Identifier) (flow.Entity, bIndex, sIndex, bool) {
	idPrefix, bucketIndex := a.idPrefixAndBucketIndex(entityID)
	for k := uint64(0); k < bucketSize; k++ {
		if a.buckets[bucketIndex][k].idPrefix != idPrefix {
			continue
		}

		id, entity, linked := a.linkedEntityOf(bucketIndex, sIndex(k))
		if !linked {
			// valueIndex does not represent value entity for this key
			// this happens upon an ejection on values
			return nil, 0, 0, false
		}

		// come here to check remaining hash bits
		if id != entityID {
			continue
		}

		return entity, bucketIndex, sIndex(k), true
	}

	return nil, 0, 0, false
}

func (a ArrayBackData) idPrefixAndBucketIndex(id flow.Identifier) (uint32, bIndex) {
	// uint64(id[0:8]) used to compute bucket index for which this key (i.e., id) belongs to
	bucketIndex := binary.LittleEndian.Uint64(id[0:8]) % a.bucketNum

	// uint32(id[8:12]) used to compute a shorter key for this id to represent in memory.
	idPrefix := binary.LittleEndian.Uint32(id[8:12])

	return idPrefix, bIndex(bucketIndex)
}

func (a ArrayBackData) expiryThreshold() uint64 {
	var expiryThreshold uint64 = 0 // keyIndex(es) below expiryThreshold are eligible for eviction
	if a.keyCount > a.limit {
		expiryThreshold = a.keyCount - a.limit
	}

	return expiryThreshold
}

func (a *ArrayBackData) slotInBucket(bucketIndex bIndex, idPrefix uint32, entityID flow.Identifier) (sIndex, bool) {
	slotToUse := uint64(0)
	expiryThreshold := a.expiryThreshold()
	availableSlotCount := uint64(0)
	oldestKeyInBucket := a.keyCount + 1 // use oldest kvCount in bucket to help with random ejection mode
	for k := uint64(0); k < bucketSize; k++ {
		if a.buckets[bucketIndex][k].keyIndex < oldestKeyInBucket {
			oldestKeyInBucket = a.buckets[bucketIndex][k].keyIndex
			slotToUse = k
		}

		if a.buckets[bucketIndex][k].keyIndex <= expiryThreshold {
			// come here if slot technically expired or never assigned
			// TODO: count it as an available slot
			availableSlotCount++
			continue
		}

		// come here if kvCount above threshold AKA slot valid / new enough
		if a.buckets[bucketIndex][k].idPrefix != idPrefix {
			continue
		}

		// come here to check if kvIndex / kvOwner still linked
		id, _, linked := a.linkedEntityOf(bucketIndex, sIndex(k))
		if !linked {
			availableSlotCount++
			slotToUse = k
			continue
		}

		// come here to check remaining hash bits
		if id != entityID {
			continue
		}

		// entity ID already exists in the bucket
		a.availableSlotHistogram[availableSlotCount]++
		return 0, false
	}

	a.availableSlotHistogram[availableSlotCount]++
	return sIndex(slotToUse), true
}

func (a ArrayBackData) ownerIndexOf(bucketIndex bIndex, slotIndex sIndex) uint64 {
	return (uint64(bucketIndex) * bucketSize) + uint64(slotIndex)
}

func (a *ArrayBackData) linkedEntityOf(bucketIndex bIndex, slot sIndex) (flow.Identifier, flow.Entity, bool) {
	if a.buckets[bucketIndex][slot].keyIndex == 0 {
		// slot never used, or recently invalidated, hence
		// does not have any linked entity
		return flow.Identifier{}, nil, false
	}

	valueIndex := a.buckets[bucketIndex][slot].valueIndex
	id, entity, owner := a.entities.Get(valueIndex)
	if a.ownerIndexOf(bucketIndex, slot) != owner {
		// TODO invalidate bucket index
		a.buckets[bucketIndex][slot].keyIndex = 0 // kvIndex / kvOwner no longer linked
		return flow.Identifier{}, nil, false
	}

	return id, entity, true
}

func (a ArrayBackData) printTelemetry() {
	for i := range a.availableSlotHistogram {
		zlog.Info().Int("i", i).Uint64("slots", a.availableSlotHistogram[i]).Msg("available")
	}
}

func (a *ArrayBackData) invalidateKey(bucketIndex bIndex, sliceIndex sIndex) {
	a.buckets[bucketIndex][sliceIndex].keyIndex = 0
}

func (a *ArrayBackData) invalidateEntity(bucketIndex bIndex, sliceIndex sIndex) {
	a.entities.Rem(a.buckets[bucketIndex][sliceIndex].valueIndex)
}
