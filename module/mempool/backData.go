package mempool

import (
	"github.com/onflow/flow-go/model/flow"
)

// BackData represents the underlying data structure used by mempool.Backend
// as the core structure of maintaining data on memory pools.
//
// This interface provides fundamental operations for storing, retrieving, and removing data structures,
// but it does not support modifying mutating already stored data. If modifications to the stored data is required,
// use [MutableBackData] instead.
//
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

	// GetWithInit returns the given entity from the backdata. If the entity does not exist, it creates a new entity
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
