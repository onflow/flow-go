package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// Backdata implements a generic memory pool backed by a Go map.
type Backdata struct {
	entities map[flow.Identifier]flow.Entity
}

func NewBackdata() Backdata {
	bd := Backdata{
		entities: make(map[flow.Identifier]flow.Entity),
	}
	return bd
}

// Has checks if we already contain the item with the given hash.
func (b *Backdata) Has(entityID flow.Identifier) bool {
	_, exists := b.entities[entityID]
	return exists
}

// Add adds the given item to the pool.
func (b *Backdata) Add(entityID flow.Identifier, entity flow.Entity) bool {
	_, exists := b.entities[entityID]
	if exists {
		return false
	}
	b.entities[entityID] = entity
	return true
}

// Rem will remove the item with the given hash.
func (b *Backdata) Rem(entityID flow.Identifier) (flow.Entity, bool) {
	entity, exists := b.entities[entityID]
	if !exists {
		return nil, false
	}
	delete(b.entities, entityID)
	return entity, true
}

// Adjust will adjust the value item using the given function if the given key can be found.
// Returns a bool which indicates whether the value was updated as well as the updated value
func (b *Backdata) Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool) {
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
func (b *Backdata) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	entity, exists := b.entities[entityID]
	if !exists {
		return nil, false
	}
	return entity, true
}

// Size will return the size of the backend.
func (b *Backdata) Size() uint {
	return uint(len(b.entities))
}

// All returns all entities from the pool.
func (b *Backdata) All() []flow.Entity {
	entities := make([]flow.Entity, 0, len(b.entities))
	for _, item := range b.entities {
		entities = append(entities, item)
	}
	return entities
}

// Clear removes all entities from the pool.
func (b *Backdata) Clear() {
	b.entities = make(map[flow.Identifier]flow.Entity)
}

// Hash will use a merkle root hash to hash all items.
func (b *Backdata) Hash() flow.Identifier {
	return flow.MerkleRoot(flow.GetIDs(b.All())...)
}
