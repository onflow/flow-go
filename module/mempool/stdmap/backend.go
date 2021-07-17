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
	bs1 := binstat.EnterTime("~4lock:r:Backend.Has")
	bs1.Run(func() {
		b.RLock()
	})
	bs1.Leave()

	var has bool
	bs2 := binstat.EnterTime("~7Backend.Has")
	bs2.Run(func() {
		defer b.RUnlock()
		has = b.Backdata.Has(entityID)
	})
	bs2.Leave()
	return has
}

// Add adds the given item to the pool.
func (b *Backend) Add(entity flow.Entity) bool {
	var entityID flow.Identifier
	bs0 := binstat.EnterTime("~7Backend.Add/ID")
	bs0.Run(func() {
		entityID = entity.ID() // this expensive operation done OUTSIDE of lock :-)
	})
	bs0.Leave()

	bs1 := binstat.EnterTime("~4lock:w:Backend.Add")
	bs1.Run(func() {
		b.Lock()
	})
	bs1.Leave()

	var added bool
	bs2 := binstat.EnterTime("~7Backend.Add")
	bs2.Run(func() {
		defer b.Unlock()
		added = b.Backdata.Add(entityID, entity)
		b.reduce()
	})
	bs2.Leave()
	return added
}

// Rem will remove the item with the given hash.
func (b *Backend) Rem(entityID flow.Identifier) bool {
	bs1 := binstat.EnterTime("~4lock:w:Backend.Rem")
	bs1.Run(func() {
		b.Lock()
	})
	bs1.Leave()

	var removed bool
	bs2 := binstat.EnterTime("~7Backend.Rem")
	bs2.Run(func() {
		defer b.Unlock()
		_, removed = b.Backdata.Rem(entityID)
	})
	bs2.Leave()
	return removed
}

// Adjust will adjust the value item using the given function if the given key can be found.
// Returns a bool which indicates whether the value was updated.
func (b *Backend) Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool) {
	bs1 := binstat.EnterTime("~4lock:w:Backend.Adjust")
	bs1.Run(func() {
		b.Lock()
	})
	bs1.Leave()

	var entity flow.Entity
	var wasUpdated bool
	bs2 := binstat.EnterTime("~7Backend.Adjust")
	bs2.Run(func() {
		defer b.Unlock()
		entity, wasUpdated = b.Backdata.Adjust(entityID, f)
	})
	bs2.Leave()
	return entity, wasUpdated
}

// ByID returns the given item from the pool.
func (b *Backend) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	bs1 := binstat.EnterTime("~4lock:r:Backend.ByID")
	bs1.Run(func() {
		b.RLock()
	})
	bs1.Leave()

	var entity flow.Entity
	var exists bool
	bs2 := binstat.EnterTime("~7Backend.ByID")
	bs2.Run(func() {
		defer b.RUnlock()
		entity, exists = b.Backdata.ByID(entityID)
	})
	bs2.Leave()
	return entity, exists
}

// Run executes a function giving it exclusive access to the backdata
func (b *Backend) Run(f func(backdata map[flow.Identifier]flow.Entity) error) error {
	bs1 := binstat.EnterTime("~4lock:w:Backend.Run")
	bs1.Run(func() {
		b.Lock()
	})
	bs1.Leave()

	var err error
	bs2 := binstat.EnterTime("~7Backend.Run")
	bs2.Run(func() {
		defer b.Unlock()
		err = f(b.Backdata.entities)
		b.reduce()
	})
	bs2.Leave()
	return err
}

// Size will return the size of the backend.
func (b *Backend) Size() uint {
	bs1 := binstat.EnterTime("~4lock:r:Backend.Size")
	bs1.Run(func() {
		b.RLock()
	})
	bs1.Leave()

	var size uint
	bs2 := binstat.EnterTime("~7Backend.Size")
	bs2.Run(func() {
		defer b.RUnlock()
		size = b.Backdata.Size()
	})
	bs2.Leave()
	return size
}

// Limit returns the maximum number of items allowed in the backend.
func (b *Backend) Limit() uint {
	return b.limit
}

// All returns all entities from the pool.
func (b *Backend) All() []flow.Entity {
	bs1 := binstat.EnterTime("~4lock:r:Backend.All")
	bs1.Run(func() {
		b.RLock()
	})
	bs1.Leave()

	defer b.RUnlock()
	return b.Backdata.All()
}

// Clear removes all entities from the pool.
func (b *Backend) Clear() {
	bs1 := binstat.EnterTime("~4lock:w:Backend.Clear")
	bs1.Run(func() {
		b.Lock()
	})
	bs1.Leave()

	bs2 := binstat.EnterTime("~7Backend.Clear")
	bs2.Run(func() {
		defer b.Unlock()
		b.Backdata.Clear()
	})
	bs2.Leave()
}

// Hash will use a merkle root hash to hash all items.
func (b *Backend) Hash() flow.Identifier {
	bs1 := binstat.EnterTime("~4lock:r:Backend.Hash")
	bs1.Run(func() {
		b.RLock()
	})
	bs1.Leave()

	var identifier flow.Identifier
	bs2 := binstat.EnterTime("~7Backend.Hash")
	bs2.Run(func() {
		defer b.RUnlock()
		identifier = b.Backdata.Hash()
	})
	bs2.Leave()
	return identifier
}

// RegisterEjectionCallbacks adds the provided OnEjection callbacks
func (b *Backend) RegisterEjectionCallbacks(callbacks ...mempool.OnEjection) {
	bs1 := binstat.EnterTime("~4lock:w:Backend.RegisterEjectionCallbacks")
	bs1.Run(func() {
		b.Lock()
	})
	bs1.Leave()

	bs2 := binstat.EnterTime("~7Backend.RegisterEjectionCallbacks")
	bs2.Run(func() {
		defer b.Unlock()
		b.ejectionCallbacks = append(b.ejectionCallbacks, callbacks...)
	})
	bs2.Leave()
}

// reduce will reduce the size of the kept entities until we are within the
// configured memory pool size limit.
func (b *Backend) reduce() {
	bs := binstat.EnterTime("~7Backend.reduce")
	defer bs.Leave()

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
