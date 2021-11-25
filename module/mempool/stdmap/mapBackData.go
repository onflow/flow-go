package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// MapBackData implements a map-based generic memory pool backed by a Go map.
type MapBackData struct {
	entities map[flow.Identifier]flow.Entity
}

func NewMapBackData() MapBackData {
	bd := MapBackData{
		entities: make(map[flow.Identifier]flow.Entity),
	}
	return bd
}

// Has checks if we already contain the item with the given hash.
func (b *MapBackData) Has(entityID flow.Identifier) bool {
	_, exists := b.entities[entityID]
	return exists
}

// Add adds the given item to the pool.
func (b *MapBackData) Add(entityID flow.Identifier, entity flow.Entity) bool {
	_, exists := b.entities[entityID]
	if exists {
		return false
	}
	b.entities[entityID] = entity
	return true
}

// Rem will remove the item with the given hash.
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

// ByID returns the given item from the pool.
func (b *MapBackData) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	entity, exists := b.entities[entityID]
	if !exists {
		return nil, false
	}
	return entity, true
}

// Size will return the size of the backend.
func (b *MapBackData) Size() uint {
	return uint(len(b.entities))
}

// All returns all entities from the pool.
func (b *MapBackData) All() []flow.Entity {
	entities := make([]flow.Entity, 0, len(b.entities))
	for _, item := range b.entities {
		entities = append(entities, item)
	}
	return entities
}

// Clear removes all entities from the pool.
func (b *MapBackData) Clear() {
	b.entities = make(map[flow.Identifier]flow.Entity)
}

// Hash will use a merkle root hash to hash all items.
func (b *MapBackData) Hash() flow.Identifier {
	return flow.MerkleRoot(flow.GetIDs(b.All())...)
}

// Run executes a function giving it exclusive access to the backdata
func (b *MapBackData) Run(f func(backdata map[flow.Identifier]flow.Entity) error) error {
	return f(b.entities)
}
