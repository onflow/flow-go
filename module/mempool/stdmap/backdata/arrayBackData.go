package backdata

import (
	"encoding/binary"
	"math/rand"

	"github.com/onflow/flow-go/model/flow"
)

type EjectionMode string

const (
	bucketSize     = uint64(16)
	RandomEjection = EjectionMode("random-ejection")
	LRUEjection    = EjectionMode("lru-ejection")
)

type key struct {
	keyIndex   uint64 // slot age
	valueIndex uint32 // link to key value pair
	idPrefix   uint32 // 32bits of key sha256
}

// keyBucket represents a bucket of keys.
type keyBucket [bucketSize]key

type cachedEntity struct {
	id     flow.Identifier
	owner  uint64
	entity flow.Entity
}

// ArrayBackData implements an array-based generic memory pool backed by a fixed size array.
type ArrayBackData struct {
	limit        uint64
	overLimit    uint64
	keyCount     uint64 // total number of non-expired key-values
	bucketNum    uint64 // total number of buckets (i.e., size of buckets)
	ejectionMode EjectionMode
	buckets      []keyBucket
	entities     []cachedEntity
}

func NewArrayBackData(limit uint32, oversizeFactor uint32, mode EjectionMode) ArrayBackData {
	// total buckets
	bucketNum := uint64(limit*oversizeFactor) / bucketSize

	bd := ArrayBackData{
		limit:        uint64(limit),
		overLimit:    uint64(limit * oversizeFactor),
		buckets:      make([]keyBucket, bucketNum),
		entities:     make([]cachedEntity, limit),
		ejectionMode: mode,
	}

	return bd
}

// Has checks if we already contain the item with the given hash.
func (a *ArrayBackData) Has(entityID flow.Identifier) bool {
	return false
}

// Add adds the given item to the pool.
func (a *ArrayBackData) Add(entityID flow.Identifier, entity flow.Entity) bool {
	return false
}

// Rem will remove the item with the given hash.
func (a *ArrayBackData) Rem(entityID flow.Identifier) (flow.Entity, bool) {
	return nil, false
}

// Adjust will adjust the value item using the given function if the given key can be found.
// Returns a bool which indicates whether the value was updated as well as the updated value
func (a *ArrayBackData) Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool) {
	return nil, false
}

// ByID returns the given item from the pool.
func (a *ArrayBackData) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	return nil, false
}

// Size will return the size of the backend.
func (a *ArrayBackData) Size() uint {
	return uint(0)
}

// All returns all entities from the pool.
func (a *ArrayBackData) All() map[flow.Identifier]flow.Entity {
	return nil
}

// Clear removes all entities from the pool.
func (a *ArrayBackData) Clear() {

}

// Hash will use a merkle root hash to hash all items.
func (a *ArrayBackData) Hash() flow.Identifier {
	return flow.MerkleRoot(flow.GetIDs(a.All())...)
}

func (a *ArrayBackData) add(entityID flow.Identifier, entity flow.Entity) bool {
	idPrefix, bucketIndex := a.idPrefixAndBucketIndex(entityID)
	slotToUse, unique := a.slotInBucket(bucketIndex, idPrefix, entityID)
	if !unique {
		// entityID already exists
		return false
	}

	entityIndex := a.valueIndexForEntity()
	a.keyCount++
	a.buckets[bucketIndex][slotToUse].keyIndex = a.keyCount
	a.buckets[bucketIndex][slotToUse].valueIndex = entityIndex
	a.buckets[bucketIndex][slotToUse].idPrefix = idPrefix
	a.entities[entityIndex].entity = entity
	a.entities[entityIndex].owner = (bucketIndex * bucketSize) + slotToUse
	return true
}

func (a ArrayBackData) idPrefixAndBucketIndex(id flow.Identifier) (uint32, uint64) {
	// uint64(id[0:8]) used to compute bucket index for which this key (i.e., id) belongs to
	bucketIndex := binary.LittleEndian.Uint64(id[0:8]) % a.bucketNum

	// uint32(id[8:12]) used to compute a shorter key for this id to represent in memory.
	idPrefix := binary.LittleEndian.Uint32(id[8:12])

	return idPrefix, bucketIndex
}

func (a ArrayBackData) expiryThreshold() uint64 {
	var expiryThreshold uint64 = 0 // keyIndex(es) below expiryThreshold are eligible for eviction
	if a.keyCount > a.limit {
		expiryThreshold = a.keyCount - a.limit
	}

	return expiryThreshold
}

func (a *ArrayBackData) slotInBucket(bucketIndex uint64, idPrefix uint32, entityID flow.Identifier) (uint64, bool) {
	slotToUse := uint64(0)
	expiryThreshold := a.expiryThreshold()

	oldestKeyInBucket := a.keyCount + 1 // use oldest kvCount in bucket to help with random ejection mode
	for k := uint64(0); k < bucketSize; k++ {
		if a.buckets[bucketIndex][k].keyIndex < oldestKeyInBucket {
			oldestKeyInBucket = a.buckets[bucketIndex][k].keyIndex
			slotToUse = k
		}

		if a.buckets[bucketIndex][k].keyIndex <= expiryThreshold {
			// come here if slot technically expired or never assigned
			// TODO: count it as an available slot
			continue
		}

		// come here if kvCount above threshold AKA slot valid / new enough
		if a.buckets[bucketIndex][k].idPrefix != idPrefix {
			continue
		}

		// come here to check if kvIndex / kvOwner still linked
		kvIndex := a.buckets[bucketIndex][k].keyIndex
		kvOwner := a.entities[kvIndex].owner
		if ((bucketIndex * bucketSize) + k) != kvOwner {
			a.buckets[bucketIndex][k].keyIndex = 0 // kvIndex / kvOwner no longer linked
			continue
		}

		// come here to check remaining hash bits
		if a.entities[kvIndex].id != entityID {
			continue
		}

		// entity ID already exists in the bucket
		return 0, false
	}

	return slotToUse, true
}

func (a *ArrayBackData) valueIndexForEntity() uint32 {
	if a.keyCount < a.limit {
		// we are not over the limit yet.
		return uint32(a.keyCount)
	} else {
		// array back data is full
		if a.ejectionMode == RandomEjection {
			// ejecting a random entity
			return uint32(rand.Intn(int(a.limit)))
		} else {
			// ejecting oldest entity
			return uint32(a.keyCount % a.limit)
		}
	}
}
