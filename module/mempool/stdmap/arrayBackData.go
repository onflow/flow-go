package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

const bucketSize = 16
const bucketNum = 1000

type cachedEntity struct {
	identifier flow.Identifier
	entity     flow.Entity
}

// ArrayBackData implements an array-based generic memory pool backed by a fixed size array.
type ArrayBackData struct {
	flc [bucketSize * bucketNum]cachedEntity // first-level cache
}

func NewArrayBackData() ArrayBackData {
	bd := ArrayBackData{}
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

// Run executes a function giving it exclusive access to the backdata
func (a *ArrayBackData) Run(f func(backdata map[flow.Identifier]flow.Entity) error) error {
	return f(b.entities)
}
