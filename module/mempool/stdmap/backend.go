// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"math"
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
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
	_, ok := b.entities[entityID]
	return ok
}

// Add adds the given item to the pool.
func (b *Backdata) Add(entity flow.Entity) error {
	entityID := entity.ID()
	_, ok := b.entities[entityID]
	if ok {
		return mempool.ErrEntityAlreadyExists
	}
	b.entities[entityID] = entity
	return nil
}

// Rem will remove the item with the given hash.
func (b *Backdata) Rem(entityID flow.Identifier) bool {
	_, ok := b.entities[entityID]
	if !ok {
		return false
	}
	delete(b.entities, entityID)
	return true
}

// ByID returns the given item from the pool.
func (b *Backdata) ByID(entityID flow.Identifier) (flow.Entity, error) {
	_, ok := b.entities[entityID]
	if !ok {
		return nil, mempool.ErrEntityNotFound
	}
	entity := b.entities[entityID]
	return entity, nil
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
	return b.Backdata.Has(entityID)
}

// Add adds the given item to the pool.
func (b *Backend) Add(entity flow.Entity) error {
	b.Lock()
	defer b.Unlock()
	err := b.Backdata.Add(entity)
	b.reduce()
	return err
}

// Rem will remove the item with the given hash.
func (b *Backend) Rem(entityID flow.Identifier) bool {
	b.Lock()
	defer b.Unlock()
	return b.Backdata.Rem(entityID)
}

// ByID returns the given item from the pool.
func (b *Backend) ByID(entityID flow.Identifier) (flow.Entity, error) {
	b.RLock()
	defer b.RUnlock()
	return b.Backdata.ByID(entityID)
}

// Run executes a function giving it exclusive access to the backdata
func (b *Backend) Run(f func(backdata *Backdata) error) error {
	b.Lock()
	defer b.Unlock()
	err := f(&b.Backdata)
	b.reduce()
	return err
}

// Size will return the size of the backend.
func (b *Backend) Size() uint {
	b.RLock()
	defer b.RUnlock()
	return b.Backdata.Size()
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
