// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"math"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
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

// Backend provides synchronized access to a backdata
type Backend struct {
	sync.RWMutex
	Backdata
	ejectionTrigger             uint
	eject             EjectFunc
	ejectionCallbacks []mempool.OnEjection
}

// NewBackend creates a new memory pool backend.
// This is using EjectTrueRandomFast()
func NewBackend(options ...OptionFunc) *Backend {
	b := Backend{
		Backdata:          NewBackdata(),
		ejectionTrigger:             uint(math.MaxUint32),
		eject:             EjectTrueRandomFast,
		ejectionCallbacks: nil,
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
	_, removed := b.Backdata.Rem(entityID)
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
	b.reduce()
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
	return b.ejectionTrigger
}

// All returns all entities from the pool.
func (b *Backend) All() []flow.Entity {
	b.RLock()
	defer b.RUnlock()
	return b.Backdata.All()
}

// Clear removes all entities from the pool.
func (b *Backend) Clear() {
	b.Lock()
	defer b.Unlock()
	b.Backdata.Clear()
}

// Hash will use a merkle root hash to hash all items.
func (b *Backend) Hash() flow.Identifier {
	b.RLock()
	defer b.RUnlock()
	return b.Backdata.Hash()
}

// RegisterEjectionCallbacks adds the provided OnEjection callbacks
func (b *Backend) RegisterEjectionCallbacks(callbacks ...mempool.OnEjection) {
	b.Lock()
	defer b.Unlock()
	b.ejectionCallbacks = append(b.ejectionCallbacks, callbacks...)
}

// reduce will reduce the size of the kept entities until we are within the
// configured memory pool size limit.
func (b *Backend) reduce() {
	// we keep reducing the cache size until we are at limit again
	// this was a loop, but the loop is now in EjectTrueRandomFast()
	// the ejections are batched, so this call to eject() may not actually
	// do anything until the batch threshold is reached (currently 128)
	if len(b.entities) > int(b.ejectionTrigger) {
		// get the key from the eject function
		// revert to prior commits if this eject function is not
		// EjectTrueRandomFast
		// we don't do anything if there is an error
		_, _, _ = b.eject(b)
	}
}
