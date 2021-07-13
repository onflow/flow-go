// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"math"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/utils/binstat"
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

// Backend provides synchronized access to a backdata
type Backend struct {
	sync.RWMutex
	Backdata
	limit             uint
	eject             EjectFunc
	ejectionCallbacks []mempool.OnEjection
}

// NewBackend creates a new memory pool backend.
func NewBackend(options ...OptionFunc) *Backend {
	b := Backend{
		Backdata:          NewBackdata(),
		limit:             uint(math.MaxUint32),
		eject:             EjectTrueRandom,
		ejectionCallbacks: nil,
	}
	for _, option := range options {
		option(&b)
	}
	return &b
}

// Has checks if we already contain the item with the given hash.
func (b *Backend) Has(entityID flow.Identifier) bool {
	p1 := binstat.EnterTime("~4lock:r:Backend.Has", "")
	b.RLock()
	binstat.Leave(p1)
	p2 := binstat.EnterTime("~7Backend.Has", "")
	defer binstat.Leave(p2)
	defer b.RUnlock()
	has := b.Backdata.Has(entityID)
	return has
}

// Add adds the given item to the pool.
func (b *Backend) Add(entity flow.Entity) bool {
	p0 := binstat.EnterTime("~7Backend.Add/ID", "")
	entityID := entity.ID() // this expensive operation done OUTSIDE of lock :-)
	binstat.Leave(p0)

	p1 := binstat.EnterTime("~4lock:w:Backend.Add", "")
	b.Lock()
	binstat.Leave(p1)
	p2 := binstat.EnterTime("~7Backend.Add", "")
	defer binstat.Leave(p2)
	defer b.Unlock()
	added := b.Backdata.Add(entityID, entity)
	b.reduce()
	return added
}

// Rem will remove the item with the given hash.
func (b *Backend) Rem(entityID flow.Identifier) bool {
	p1 := binstat.EnterTime("~4lock:w:Backend.Rem", "")
	b.Lock()
	binstat.Leave(p1)
	p2 := binstat.EnterTime("~7Backend.Rem", "")
	defer binstat.Leave(p2)
	defer b.Unlock()
	_, removed := b.Backdata.Rem(entityID)
	return removed
}

// Adjust will adjust the value item using the given function if the given key can be found.
// Returns a bool which indicates whether the value was updated.
func (b *Backend) Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool) {
	p1 := binstat.EnterTime("~4lock:w:Backend.Adjust", "")
	b.Lock()
	binstat.Leave(p1)
	p2 := binstat.EnterTime("~7Backend.Adjust", "")
	defer binstat.Leave(p2)
	defer b.Unlock()
	return b.Backdata.Adjust(entityID, f)
}

// ByID returns the given item from the pool.
func (b *Backend) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	p1 := binstat.EnterTime("~4lock:r:Backend.ByID", "")
	b.RLock()
	binstat.Leave(p1)
	p2 := binstat.EnterTime("~7Backend.ByID", "")
	defer binstat.Leave(p2)
	defer b.RUnlock()
	entity, exists := b.Backdata.ByID(entityID)
	return entity, exists
}

// Run executes a function giving it exclusive access to the backdata
func (b *Backend) Run(f func(backdata map[flow.Identifier]flow.Entity) error) error {
	p1 := binstat.EnterTime("~4lock:w:Backend.Run", "")
	b.Lock()
	binstat.Leave(p1)
	p2 := binstat.EnterTime("~7Backend.Run", "")
	defer binstat.Leave(p2)
	defer b.Unlock()
	err := f(b.Backdata.entities)
	b.reduce()
	return err
}

// Size will return the size of the backend.
func (b *Backend) Size() uint {
	p1 := binstat.EnterTime("~4lock:r:Backend.Size", "")
	b.RLock()
	binstat.Leave(p1)
	p2 := binstat.EnterTime("~7Backend.Size", "")
	defer binstat.Leave(p2)
	defer b.RUnlock()
	return b.Backdata.Size()
}

// Limit returns the maximum number of items allowed in the backend.
func (b *Backend) Limit() uint {
	return b.limit
}

// All returns all entities from the pool.
func (b *Backend) All() []flow.Entity {
	p1 := binstat.EnterTime("~4lock:r:Backend.All", "")
	b.RLock()
	binstat.Leave(p1)
	defer b.RUnlock()
	return b.Backdata.All()
}

// Clear removes all entities from the pool.
func (b *Backend) Clear() {
	p := binstat.EnterTime("~4lock:w:Backend.Clear", "")
	b.Lock()
	binstat.Leave(p)
	p2 := binstat.EnterTime("~7Backend.Clear", "")
	defer binstat.Leave(p2)
	defer b.Unlock()
	b.Backdata.Clear()
}

// Hash will use a merkle root hash to hash all items.
func (b *Backend) Hash() flow.Identifier {
	p1 := binstat.EnterTime("~4lock:r:Backend.Hash", "")
	b.RLock()
	binstat.Leave(p1)
	p2 := binstat.EnterTime("~7Backend.Hash", "")
	defer binstat.Leave(p2)
	defer b.RUnlock()
	return b.Backdata.Hash()
}

// RegisterEjectionCallbacks adds the provided OnEjection callbacks
func (b *Backend) RegisterEjectionCallbacks(callbacks ...mempool.OnEjection) {
	p1 := binstat.EnterTime("~4lock:w:Backend.RegisterEjectionCallbacks", "")
	b.Lock()
	binstat.Leave(p1)
	p2 := binstat.EnterTime("~7Backend.RegisterEjectionCallbacks", "")
	defer binstat.Leave(p2)
	defer b.Unlock()
	b.ejectionCallbacks = append(b.ejectionCallbacks, callbacks...)
}

// reduce will reduce the size of the kept entities until we are within the
// configured memory pool size limit.
func (b *Backend) reduce() {
	p := binstat.EnterTime("~7Backend.reduce", "")
	defer binstat.Leave(p)

	// we keep reducing the cache size until we are at limit again
	for len(b.entities) > int(b.limit) {

		// get the key from the eject function
		key, _ := b.eject(b.entities)

		// if the key is not actually part of the map, use stupid fallback eject
		entity, ok := b.entities[key]
		if !ok {
			key, _ = EjectFakeRandom(b.entities)
		}

		// remove the key
		delete(b.entities, key)

		// notify callback
		for _, callback := range b.ejectionCallbacks {
			callback(entity)
		}
	}
}
