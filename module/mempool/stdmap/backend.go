// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
)

// backend implements a generic memory pool backed by a Go map.
type backend struct {
	sync.RWMutex
	entities map[flow.Identifier]flow.Entity
}

// newBackend creates a new memory pool backend.
func newBackend() *backend {
	b := &backend{
		entities: make(map[flow.Identifier]flow.Entity),
	}
	return b
}

// Has checks if we already contain the item with the given hash.
func (b *backend) Has(id flow.Identifier) bool {
	b.RLock()
	defer b.RUnlock()
	_, ok := b.entities[id]
	return ok
}

// Add adds the given item to the pool.
func (b *backend) Add(entity flow.Entity) error {
	b.Lock()
	defer b.Unlock()
	id := entity.ID()
	_, ok := b.entities[id]
	if ok {
		return mempool.ErrEntityAlreadyExists
	}
	b.entities[id] = entity
	return nil
}

// Rem will remove the item with the given hash.
func (b *backend) Rem(id flow.Identifier) bool {
	b.Lock()
	defer b.Unlock()
	_, ok := b.entities[id]
	if !ok {
		return false
	}
	delete(b.entities, id)
	return true
}

// Get returns the given item from the pool.
func (b *backend) Get(id flow.Identifier) (flow.Entity, error) {
	b.RLock()
	defer b.RUnlock()
	_, ok := b.entities[id]
	if !ok {
		return nil, mempool.ErrEntityNotFound
	}
	coll := b.entities[id]
	return coll, nil
}

// Size will return the size of the backend.
func (b *backend) Size() uint {
	b.RLock()
	defer b.RUnlock()
	return uint(len(b.entities))
}

// All returns all entities from the pool.
func (b *backend) All() []flow.Entity {
	b.RLock()
	defer b.RUnlock()
	entities := make([]flow.Entity, 0, len(b.entities))
	for _, item := range b.entities {
		entities = append(entities, item)
	}
	return entities
}

// Hash will use a merkle root hash to hash all items.
func (b *backend) Hash() flow.Identifier {
	b.RLock()
	defer b.RUnlock()
	return flow.MerkleRoot(flow.GetIDs(b.All())...)
}
