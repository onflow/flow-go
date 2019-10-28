// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"bytes"
	"encoding/hex"
	"sort"
	"sync"

	"github.com/dchest/siphash"
	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/pkg/model/collection"
)

// Mempool implements the collections memory pool of the consensus nodes, used to
// store guaranteed collections and to generate block payloads.
type Mempool struct {
	sync.Mutex
	collections map[string]*collection.GuaranteedCollection
}

// New creates a new memory pool for guaranteed collections.
func New() (*Mempool, error) {
	m := &Mempool{
		collections: make(map[string]*collection.GuaranteedCollection),
	}
	return m, nil
}

// Has checks if we know already know the guaranteed collection of the given
// hash.
func (m *Mempool) Has(hash []byte) bool {
	m.Lock()
	defer m.Unlock()
	key := hex.EncodeToString(hash)
	_, ok := m.collections[key]
	return ok
}

// Add adds the given guaranteed collection to the pool.
func (m *Mempool) Add(coll *collection.GuaranteedCollection) error {
	m.Lock()
	defer m.Unlock()
	key := hex.EncodeToString(coll.Hash)
	_, ok := m.collections[key]
	if ok {
		return errors.Errorf("collection already known (%s)", key)
	}
	m.collections[key] = coll
	return nil
}

// Get returns the given collection from the pool.
func (m *Mempool) Get(hash []byte) (*collection.GuaranteedCollection, error) {
	m.Lock()
	defer m.Unlock()
	key := hex.EncodeToString(hash)
	coll, ok := m.collections[key]
	if !ok {
		return nil, errors.Errorf("could not find collection (%s)", key)
	}
	return coll, nil
}

// Hash returns a hash of all collections in the mempool.
func (m *Mempool) Hash() []byte {
	m.Lock()
	defer m.Unlock()
	hash := siphash.New([]byte("flowcollmempoolx"))
	collections := m.all()
	sort.Slice(collections, func(i int, j int) bool {
		return bytes.Compare(collections[i].Hash, collections[j].Hash) < 0
	})
	for _, coll := range collections {
		_, _ = hash.Write(coll.Hash)
	}
	return hash.Sum(nil)
}

// Size will return the size of the mempool.
func (m *Mempool) Size() uint {
	m.Lock()
	defer m.Unlock()
	return uint(len(m.collections))
}

// All returns all collections from the pool.
func (m *Mempool) All() []*collection.GuaranteedCollection {
	m.Lock()
	defer m.Unlock()
	return m.all()
}

func (m *Mempool) all() []*collection.GuaranteedCollection {
	collections := make([]*collection.GuaranteedCollection, 0, len(m.collections))
	for _, coll := range m.collections {
		collections = append(collections, coll)
	}
	return collections
}
