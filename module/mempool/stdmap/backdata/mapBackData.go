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

// Has checks if backdata already contains the entity with the given identifier.
func (b MapBackData) Has(entityID flow.Identifier) bool {
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

// Rem removes the entity with the given identifier.
func (b *MapBackData) Rem(entityID flow.Identifier) (flow.Entity, bool) {
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

// ByID returns the given entity from the backdata.
func (b MapBackData) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	entity, exists := b.entities[entityID]
	if !exists {
		return nil, false
	}
	return entity, true
}

// Size returns the size of the backdata, i.e., total number of stored (entityId, entity)
func (b MapBackData) Size() uint {
	return uint(len(b.entities))
}

// All returns all entities stored in the backdata.
func (b MapBackData) All() map[flow.Identifier]flow.Entity {
	entities := make(map[flow.Identifier]flow.Entity)
	for entityID, entity := range b.entities {
		entities[entityID] = entity
	}
	return entities
}

// Identifiers returns the list of identifiers of entities stored in the backdata.
func (b MapBackData) Identifiers() flow.IdentifierList {
	ids := make(flow.IdentifierList, len(b.entities))
	i := 0
	for entityID := range b.entities {
		ids[i] = entityID
		i++
	}
	return ids
}

// Entities returns the list of entities stored in the backdata.
func (b MapBackData) Entities() []flow.Entity {
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

// Hash returns the merkle root hash of all entities.
func (b *MapBackData) Hash() flow.Identifier {
	return flow.MerkleRoot(flow.GetIDs(b.All())...)
}
