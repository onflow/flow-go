package mempool

import (
	"github.com/onflow/flow-go/model/flow"
)

// BackData represents the underlying data structure that is utilized by mempool.Backend, as the
// core structure of maintaining data on memory-pools.
// NOTE: BackData by default is not expected to provide concurrency-safe operations. As it is just the
// model layer of the mempool, the safety against concurrent operations are guaranteed by the Backend that
// is the control layer.
type BackData interface {
	// Has checks if we already contain the item with the given hash.
	Has(entityID flow.Identifier) bool

	// Add adds the given item to the pool.
	Add(entityID flow.Identifier, entity flow.Entity) bool

	// Rem will remove the item with the given hash.
	Rem(entityID flow.Identifier) (flow.Entity, bool)

	// Adjust will adjust the value item using the given function if the given key can be found.
	// Returns a bool which indicates whether the value was updated as well as the updated value
	Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool)

	// ByID returns the given item from the pool.
	ByID(entityID flow.Identifier) (flow.Entity, bool)

	// Size will return the size of the backend.
	Size() uint

	// All returns all entities from the pool.
	All() map[flow.Identifier]flow.Entity

	// Clear removes all entities from the pool.
	Clear()

	// Hash will use a merkle root hash to hash all items.
	Hash() flow.Identifier
}
