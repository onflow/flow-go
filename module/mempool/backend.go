// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"fmt"
	"sync"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/merkle"
)

// Item defines the interface for items in the mempool.
type Item interface {
	// ID should return the canonical ID for the item.
	ID() flow.Identifier
}

// mempool implements a generic memory pool backed by a merkle tree.
type mempool struct {
	sync.RWMutex
	tree  *merkle.Tree
	items map[flow.Identifier]Item
}

// newMempool creates a new memory pool.
func newMempool() *mempool {
	m := &mempool{
		tree:  merkle.NewTree(),
		items: make(map[flow.Identifier]Item),
	}
	return m
}

// Has checks if we already contain the item with the given hash.
func (m *mempool) Has(id flow.Identifier) bool {
	m.RLock()
	defer m.RUnlock()
	_, ok := m.tree.Get(id[:])
	return ok
}

// Add adds the given item to the pool.
func (m *mempool) Add(item Item) error {
	m.Lock()
	defer m.Unlock()
	id := item.ID()
	ok := m.tree.Put(id[:], nil)
	if ok {
		return fmt.Errorf("item already known (%x)", id)
	}
	m.items[id] = item
	return nil
}

// Rem will remove the item with the given hash.
func (m *mempool) Rem(id flow.Identifier) bool {
	m.Lock()
	defer m.Unlock()
	ok := m.tree.Del(id[:])
	if !ok {
		return false
	}
	delete(m.items, id)
	return true
}

// Get returns the given item from the pool.
func (m *mempool) Get(id flow.Identifier) (Item, error) {
	m.RLock()
	defer m.RUnlock()
	_, ok := m.tree.Get(id[:])
	if !ok {
		return nil, fmt.Errorf("item not known (%x)", id)
	}
	coll := m.items[id]
	return coll, nil
}

// Hash returns a hash of all items in the mempool.
func (m *mempool) Hash() crypto.Hash {
	m.RLock()
	defer m.RUnlock()
	return m.tree.Hash()
}

// Size will return the size of the mempool.
func (m *mempool) Size() uint {
	m.RLock()
	defer m.RUnlock()
	return uint(len(m.items))
}

// All returns all items from the pool.
func (m *mempool) All() []Item {
	m.RLock()
	defer m.RUnlock()
	items := make([]Item, 0, len(m.items))
	for _, item := range m.items {
		items = append(items, item)
	}
	return items
}
