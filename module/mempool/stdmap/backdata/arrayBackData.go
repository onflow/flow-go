package backdata

import (
	"github.com/onflow/flow-go/model/flow"
)

const bucketSize = uint64(16)

type slotStruct struct {
	kvCount uint64 // slot age
	kvIndex uint32 // link to key value pair
	sum32   uint32 // 32bits of key sha256
}

type keyBucket struct {
	slots [bucketSize]slotStruct
}

type cachedEntity struct {
	id     flow.Identifier
	owner  uint64
	entity flow.Entity
}

// ArrayBackData implements an array-based generic memory pool backed by a fixed size array.
type ArrayBackData struct {
	limit   uint64
	size    uint64 // total number of non-expired key-values
	buckets []keyBucket
}

func NewArrayBackData(limit uint32, oversizeFactor uint32) ArrayBackData {
	bd := ArrayBackData{
		limit: uint64(limit * oversizeFactor),
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
