package backdata

import (
	"github.com/onflow/flow-go/model/flow"
)

// MapBackData implements a map-based generic memory BackData backed by a Go map.
// Note that this implementation is not thread-safe, and the higher-level Backend is responsible for concurrency management.
type MapBackData struct {
	// NOTE: as a BackData implementation, MapBackData must be non-blocking.
	// Concurrency management is done by overlay Backend.
	entities map[flow.Identifier]flow.Entity
}

func NewMapBackData() *MapBackData {
	bd := &MapBackData{
		entities: make(map[flow.Identifier]flow.Entity),
	}
	return bd
}

// Has checks if backdata already contains the entity with the given identifier.
func (b *MapBackData) Has(entityID flow.Identifier) bool {
	_, exists := b.entities[entityID]
	return exists
}

// Add adds the given entity to the backdata.
func (b *MapBackData) Add(entityID flow.Identifier, entity flow.Entity) bool {
	_, exists := b.entities[entityID]
	if exists {
		return false
	}
	b.entities[entityID] = entity
	return true
}

// Remove removes the entity with the given identifier.
func (b *MapBackData) Remove(entityID flow.Identifier) (flow.Entity, bool) {
	entity, exists := b.entities[entityID]
	if !exists {
		return nil, false
	}
	delete(b.entities, entityID)
	return entity, true
}

// Adjust adjusts the entity using the given function if the given identifier can be found.
// Returns a bool which indicates whether the entity was updated as well as the updated entity.
func (b *MapBackData) Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool) {
	entity, ok := b.entities[entityID]
	if !ok {
		return nil, false
	}
	newentity := f(entity)
	newentityID := newentity.ID()

	delete(b.entities, entityID)
	b.entities[newentityID] = newentity
	return newentity, true
}

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
func (b *MapBackData) AdjustWithInit(entityID flow.Identifier, adjust func(flow.Entity) flow.Entity, init func() flow.Entity) (flow.Entity, bool) {
	if b.Has(entityID) {
		return b.Adjust(entityID, adjust)
	}
	b.Add(entityID, init())
	return b.Adjust(entityID, adjust)
}

// GetWithInit returns the given entity from the backdata. If the entity does not exist, it creates a new entity
// using the factory function and stores it in the backdata.
// Args:
// - entityID: the identifier of the entity to get.
// - init: the function that initializes the entity when it is not found.
// Returns:
//   - the entity.
//   - a bool which indicates whether the entity was found (or created).
func (b *MapBackData) GetWithInit(entityID flow.Identifier, init func() flow.Entity) (flow.Entity, bool) {
	if b.Has(entityID) {
		return b.ByID(entityID)
	}
	b.Add(entityID, init())
	return b.ByID(entityID)
}

// ByID returns the given entity from the backdata.
func (b *MapBackData) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	entity, exists := b.entities[entityID]
	if !exists {
		return nil, false
	}
	return entity, true
}

// Size returns the size of the backdata, i.e., total number of stored (entityId, entity)
func (b *MapBackData) Size() uint {
	return uint(len(b.entities))
}

// All returns all entities stored in the backdata.
func (b *MapBackData) All() map[flow.Identifier]flow.Entity {
	entities := make(map[flow.Identifier]flow.Entity)
	for entityID, entity := range b.entities {
		entities[entityID] = entity
	}
	return entities
}

// Identifiers returns the list of identifiers of entities stored in the backdata.
func (b *MapBackData) Identifiers() flow.IdentifierList {
	ids := make(flow.IdentifierList, len(b.entities))
	i := 0
	for entityID := range b.entities {
		ids[i] = entityID
		i++
	}
	return ids
}

// Entities returns the list of entities stored in the backdata.
func (b *MapBackData) Entities() []flow.Entity {
	entities := make([]flow.Entity, len(b.entities))
	i := 0
	for _, entity := range b.entities {
		entities[i] = entity
		i++
	}
	return entities
}

// Clear removes all entities from the backdata.
func (b *MapBackData) Clear() {
	b.entities = make(map[flow.Identifier]flow.Entity)
}
