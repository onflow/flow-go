package backdata

import (
	"encoding/binary"

	"github.com/onflow/flow-go/model/flow"
)

const bucketSize = uint64(16)

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
	limit     uint64
	size      uint64 // total number of non-expired key-values
	bucketNum uint64 // total number of buckets (i.e., size of buckets)
	buckets   []keyBucket
	entities  []cachedEntity
}

func NewArrayBackData(limit uint32, oversizeFactor uint32) ArrayBackData {
	// total buckets
	bucketNum := uint64(limit*oversizeFactor) / bucketSize

	bd := ArrayBackData{
		limit:    uint64(limit * oversizeFactor),
		buckets:  make([]keyBucket, bucketNum),
		entities: make([]cachedEntity, limit),
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

func (a *ArrayBackData) add(k flow.Identifier, v flow.Entity) (bool, error) {

}

func (a ArrayBackData) idPrefixAndBucketIndex(id flow.Identifier) (uint32, uint64) {
	// uint64(id[0:8]) used to compute bucket index for which this key (i.e., id) belongs to
	bucketIndex := binary.LittleEndian.Uint64(id[0:8]) % a.bucketNum

	// uint32(id[8:12]) used to compute a shorter key for this id to represent in memory.
	idPrefix := binary.LittleEndian.Uint32(id[8:12])

	return idPrefix, bucketIndex
}
