// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/storage/merkle"
)

// Item defines the interface for items in the mempool.
type Item interface {
	// Hash should return the canonical hash for the item.
	Hash() crypto.Hash
}

// Mempool implements a generic memory pool backed by a merkle tree.
type Mempool struct {
	sync.RWMutex
	tree  *merkle.Tree
	items map[string]Item
}

// NewMempool creates a new memory pool.
func NewMempool() (*Mempool, error) {
	m := &Mempool{
		tree:  merkle.NewTree(),
		items: make(map[string]Item),
	}
	return m, nil
}

// Has checks if we already contain the item with the given hash.
func (m *Mempool) Has(hash crypto.Hash) bool {
	m.RLock()
	defer m.RUnlock()
	_, ok := m.tree.Get(hash)
	return ok
}

// Add adds the given item to the pool.
func (m *Mempool) Add(item Item) error {
	m.Lock()
	defer m.Unlock()
	ok := m.tree.Put(item.Hash(), nil)
	if ok {
		return errors.Errorf("item already known (%x)", item.Hash())
	}
	m.items[fmt.Sprint(item.Hash())] = item
	return nil
}

// Rem will remove the item with the given hash.
func (m *Mempool) Rem(hash crypto.Hash) bool {
	m.Lock()
	defer m.Unlock()
	ok := m.tree.Del(hash)
	if !ok {
		return false
	}
	delete(m.items, fmt.Sprint(hash))
	return true
}

// Get returns the given item from the pool.
func (m *Mempool) Get(hash crypto.Hash) (Item, error) {
	m.RLock()
	defer m.RUnlock()
	_, ok := m.tree.Get(hash)
	if !ok {
		return nil, errors.Errorf("item not known (%x)", hash)
	}
	coll := m.items[fmt.Sprint(hash)]
	return coll, nil
}

// Hash returns a hash of all items in the mempool.
func (m *Mempool) Hash() crypto.Hash {
	m.RLock()
	defer m.RUnlock()
	return m.tree.Hash()
}

// Size will return the size of the mempool.
func (m *Mempool) Size() uint {
	m.RLock()
	defer m.RUnlock()
	return uint(len(m.items))
}

// All returns all items from the pool.
func (m *Mempool) All() []Item {
	m.RLock()
	defer m.RUnlock()
	items := make([]Item, 0, len(m.items))
	for _, coll := range m.items {
		items = append(items, coll)
	}
	return items
}
