package mempool

import (
	"github.com/onflow/flow-go/model/flow"
)

type Backdata interface {
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
	All() []flow.Entity

	// Clear removes all entities from the pool.
	Clear()

	// Hash will use a merkle root hash to hash all items.
	Hash() flow.Identifier
}
