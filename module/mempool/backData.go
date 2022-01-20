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
	// Has checks if backdata already contains the entity with the given identifier.
	Has(entityID flow.Identifier) bool

	// Add adds the given entity to the backdata.
	Add(entityID flow.Identifier, entity flow.Entity) bool

	// Rem removes the entity with the given identifier.
	Rem(entityID flow.Identifier) (flow.Entity, bool)

	// Adjust adjusts the entity using the given function if the given identifier can be found.
	// Returns a bool which indicates whether the entity was updated as well as the updated entity.
	Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool)

	// ByID returns the given entity from the backdata.
	ByID(entityID flow.Identifier) (flow.Entity, bool)

	// Size returns the size of the backdata, i.e., total number of stored (entityId, entity) pairs.
	Size() uint

	// All returns all entities stored in the backdata.
	All() map[flow.Identifier]flow.Entity

	// Identifiers returns the list of identifiers of entities stored in the backdata.
	Identifiers() flow.IdentifierList

	// Entities returns the list of entities stored in the backdata.
	Entities() []flow.Entity

	// Clear removes all entities from the backdata.
	Clear()

	// Hash returns the merkle root hash of all entities.
	Hash() flow.Identifier
}
