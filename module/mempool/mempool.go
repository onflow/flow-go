// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/collection"
	"github.com/dapperlabs/flow-go/storage/merkle"
)

// Mempool implements the collections memory pool of the consensus nodes, used to
// store guaranteed collections and to generate block payloads.
type Mempool struct {
	sync.RWMutex
	tree        *merkle.Tree
	collections map[string]*collection.GuaranteedCollection
}

// New creates a new memory pool for guaranteed collections.
func New() (*Mempool, error) {
	m := &Mempool{
		tree:        merkle.NewTree(),
		collections: make(map[string]*collection.GuaranteedCollection),
	}
	return m, nil
}

// Has checks if we know already know the guaranteed collection of the given
// hash.
func (m *Mempool) Has(hash crypto.Hash) bool {
	m.RLock()
	defer m.RUnlock()
	_, ok := m.tree.Get(hash)
	return ok
}

// Add adds the given guaranteed collection to the pool.
func (m *Mempool) Add(coll *collection.GuaranteedCollection) error {
	m.Lock()
	defer m.Unlock()
	ok := m.tree.Put(coll.Hash, nil)
	if ok {
		return errors.Errorf("collection already known (%x)", coll.Hash)
	}
	m.collections[fmt.Sprint(coll.Hash)] = coll
	return nil
}

// Rem will remove the collection with the given hash.
func (m *Mempool) Rem(hash crypto.Hash) bool {
	m.Lock()
	defer m.Unlock()
	ok := m.tree.Del(hash)
	if !ok {
		return false
	}
	delete(m.collections, fmt.Sprint(hash))
	return true
}

// Get returns the given collection from the pool.
func (m *Mempool) Get(hash crypto.Hash) (*collection.GuaranteedCollection, error) {
	m.RLock()
	defer m.RUnlock()
	_, ok := m.tree.Get(hash)
	if !ok {
		return nil, errors.Errorf("collection not known (%x)", hash)
	}
	coll := m.collections[fmt.Sprint(hash)]
	return coll, nil
}

// Hash returns a hash of all collections in the mempool.
func (m *Mempool) Hash() crypto.Hash {
	m.RLock()
	defer m.RUnlock()
	return m.tree.Hash()
}

// Size will return the size of the mempool.
func (m *Mempool) Size() uint {
	m.RLock()
	defer m.RUnlock()
	return uint(len(m.collections))
}

// All returns all collections from the pool.
func (m *Mempool) All() []*collection.GuaranteedCollection {
	m.RLock()
	defer m.RUnlock()
	collections := make([]*collection.GuaranteedCollection, 0, len(m.collections))
	for _, coll := range m.collections {
		collections = append(collections, coll)
	}
	return collections
}
