package backdata

import (
	"github.com/onflow/flow-go/model/flow"
)

// MapBackData implements a map-based generic memory BackData backed by a Go map.
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

// Has checks if we already contain the item with the given identifier.
func (b MapBackData) Has(entityID flow.Identifier) bool {
	_, exists := b.entities[entityID]
	return exists
}

// Add adds the given entity to the BackData.
func (b *MapBackData) Add(entityID flow.Identifier, entity flow.Entity) bool {
	_, exists := b.entities[entityID]
	if exists {
		return false
	}
	b.entities[entityID] = entity
	return true
}

// Rem will remove the entity with the given identifier.
func (b *MapBackData) Rem(entityID flow.Identifier) (flow.Entity, bool) {
	entity, exists := b.entities[entityID]
	if !exists {
		return nil, false
	}
	delete(b.entities, entityID)
	return entity, true
}

// Adjust will adjust the value item using the given function if the given key can be found.
// Returns a bool which indicates whether the value was updated as well as the updated value
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

// ByID returns the given item from the BackData.
func (b MapBackData) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	entity, exists := b.entities[entityID]
	if !exists {
		return nil, false
	}
	return entity, true
}

// Size will return the size of the BackData.
func (b MapBackData) Size() uint {
	return uint(len(b.entities))
}

// All returns all entities from the BackData.
func (b MapBackData) All() map[flow.Identifier]flow.Entity {
	entities := make(map[flow.Identifier]flow.Entity)
	for entityID, entity := range b.entities {
		entities[entityID] = entity
	}
	return entities
}

// Clear removes all entities from the BackData.
func (b *MapBackData) Clear() {
	b.entities = make(map[flow.Identifier]flow.Entity)
}

// Hash will use a merkle root hash to hash all entities.
func (b *MapBackData) Hash() flow.Identifier {
	return flow.MerkleRoot(flow.GetIDs(b.All())...)
}
