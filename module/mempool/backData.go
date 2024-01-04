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

	// Remove removes the entity with the given identifier.
	Remove(entityID flow.Identifier) (flow.Entity, bool)

	// Adjust adjusts the entity using the given function if the given identifier can be found.
	// Returns a bool which indicates whether the entity was updated as well as the updated entity.
	Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool)

	// AdjustWithInit adjusts the entity using the given function if the given identifier can be found. When the
	// entity is not found, it initializes the entity using the given init function and then applies the adjust function.
	// Args:
	// - entityID: the identifier of the entity to adjust.
	// - adjust: the function that adjusts the entity.
	// - init: the function that initializes the entity when it is not found.
	// Returns:
	//   - the adjusted entity.
	//
	// - a bool which indicates whether the entity was adjusted.
	AdjustWithInit(entityID flow.Identifier, adjust func(flow.Entity) flow.Entity, init func() flow.Entity) (flow.Entity, bool)

	// GetOrInit returns the given entity from the backdata. If the entity does not exist, it creates a new entity
	// using the factory function and stores it in the backdata.
	// Args:
	// - entityID: the identifier of the entity to get.
	// - init: the function that initializes the entity when it is not found.
	// Returns:
	//  - the entity.
	// - a bool which indicates whether the entity was found (or created).
	GetWithInit(entityID flow.Identifier, init func() flow.Entity) (flow.Entity, bool)

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
}
