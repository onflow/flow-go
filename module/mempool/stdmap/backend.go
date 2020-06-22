// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"math"
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
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
func (b *Backdata) Add(entity flow.Entity) bool {
	entityID := entity.ID()
	_, exists := b.entities[entityID]
	if exists {
		return false
	}
	b.entities[entityID] = entity
	return true
}

// Rem will remove the item with the given hash.
func (b *Backdata) Rem(entityID flow.Identifier) bool {
	_, exists := b.entities[entityID]
	if !exists {
		return false
	}
	delete(b.entities, entityID)
	return true
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

// Hash will use a merkle root hash to hash all items.
func (b *Backdata) Hash() flow.Identifier {
	return flow.MerkleRoot(flow.GetIDs(b.All())...)
}

// Backend provides synchronized access to a backdata
type Backend struct {
	sync.RWMutex
	Backdata
	limit uint
	eject EjectFunc
}

// NewBackend creates a new memory pool backend.
func NewBackend(options ...OptionFunc) *Backend {
	b := Backend{
		Backdata: NewBackdata(),
		limit:    uint(math.MaxUint32),
		eject:    EjectTrueRandom,
	}
	for _, option := range options {
		option(&b)
	}
	return &b
}

// Has checks if we already contain the item with the given hash.
func (b *Backend) Has(entityID flow.Identifier) bool {
	b.RLock()
	defer b.RUnlock()
	has := b.Backdata.Has(entityID)
	return has
}

// Add adds the given item to the pool.
func (b *Backend) Add(entity flow.Entity) bool {
	b.Lock()
	defer b.Unlock()
	added := b.Backdata.Add(entity)
	b.reduce()
	return added
}

// Rem will remove the item with the given hash.
func (b *Backend) Rem(entityID flow.Identifier) bool {
	b.Lock()
	defer b.Unlock()
	removed := b.Backdata.Rem(entityID)
	return removed
}

// Adjust will adjust the value item using the given function if the given key can be found.
// Returns a bool which indicates whether the value was updated.
func (b *Backend) Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool) {
	b.Lock()
	defer b.Unlock()
	return b.Backdata.Adjust(entityID, f)
}

// ByID returns the given item from the pool.
func (b *Backend) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	b.RLock()
	defer b.RUnlock()
	entity, exists := b.Backdata.ByID(entityID)
	return entity, exists
}

// Run executes a function giving it exclusive access to the backdata
func (b *Backend) Run(f func(backdata map[flow.Identifier]flow.Entity) error) error {
	b.Lock()
	defer b.Unlock()
	err := f(b.Backdata.entities)
	return err
}

// Size will return the size of the backend.
func (b *Backend) Size() uint {
	b.RLock()
	defer b.RUnlock()
	return b.Backdata.Size()
}

// Limit returns the maximum number of items allowed in the backend.
func (b *Backend) Limit() uint {
	return b.limit
}

// All returns all entities from the pool.
func (b *Backend) All() []flow.Entity {
	b.RLock()
	defer b.RUnlock()
	return b.Backdata.All()
}

// Hash will use a merkle root hash to hash all items.
func (b *Backend) Hash() flow.Identifier {
	b.RLock()
	defer b.RUnlock()
	return b.Backdata.Hash()
}

// reduce will reduce the size of the kept entities until we are within the
// configured memory pool size limit.
func (b *Backend) reduce() {

	// we keep reducing the cache size until we are at limit again
	for len(b.entities) > int(b.limit) {

		// get the key from the eject function
		key, _ := b.eject(b.entities)

		// if the key is not actually part of the map, use stupid fallback eject
		_, ok := b.entities[key]
		if !ok {
			key, _ = EjectFakeRandom(b.entities)
		}

		// remove the key
		delete(b.entities, key)
	}
}
