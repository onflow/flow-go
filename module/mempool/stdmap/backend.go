// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
)

// Backdata implements a generic memory pool backed by a Go map.
type Backdata struct {
	entities map[flow.Identifier]flow.Entity
}

func NewBackdata() *Backdata {
	return &Backdata{
		entities: make(map[flow.Identifier]flow.Entity),
	}
}

// Has checks if we already contain the item with the given hash.
func (b *Backdata) Has(id flow.Identifier) bool {
	_, ok := b.entities[id]
	return ok
}

// Add adds the given item to the pool.
func (b *Backdata) Add(entity flow.Entity) error {
	id := entity.ID()
	_, ok := b.entities[id]
	if ok {
		return mempool.ErrEntityAlreadyExists
	}
	b.entities[id] = entity
	return nil
}

// Rem will remove the item with the given hash.
func (b *Backdata) Rem(id flow.Identifier) bool {
	_, ok := b.entities[id]
	if !ok {
		return false
	}
	delete(b.entities, id)
	return true
}

// ByID returns the given item from the pool.
func (b *Backdata) ByID(id flow.Identifier) (flow.Entity, error) {
	_, ok := b.entities[id]
	if !ok {
		return nil, mempool.ErrEntityNotFound
	}
	coll := b.entities[id]
	return coll, nil
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

// Backend provides synchronized access to a backend
type Backend struct {
	*Backdata
	sync.RWMutex
}

// NewBackend creates a new memory pool backend.
func NewBackend() *Backend {
	b := &Backend{
		Backdata: NewBackdata(),
	}
	return b
}

// Has checks if we already contain the item with the given hash.
func (b *Backend) Has(id flow.Identifier) bool {
	b.RLock()
	defer b.RUnlock()
	return b.Backdata.Has(id)
}

// Add adds the given item to the pool.
func (b *Backend) Add(entity flow.Entity) error {
	b.Lock()
	defer b.Unlock()
	return b.Backdata.Add(entity)
}

// Rem will remove the item with the given hash.
func (b *Backend) Rem(id flow.Identifier) bool {
	b.Lock()
	defer b.Unlock()
	return b.Backdata.Rem(id)
}

// ByID returns the given item from the pool.
func (b *Backend) ByID(id flow.Identifier) (flow.Entity, error) {
	b.RLock()
	defer b.RUnlock()
	return b.Backdata.ByID(id)
}

// Run fetches the given item from the pool and runs given function on it, returning the entity after
func (b *Backend) Run(f func(backdata *Backdata) error) error {
	b.RLock()
	defer b.RUnlock()

	err := f(b.Backdata)
	if err != nil {
		return err
	}
	return nil
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
