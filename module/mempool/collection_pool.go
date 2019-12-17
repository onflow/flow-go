// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/collection"
)

// CollectionPool implements the collections memory pool of the consensus nodes,
// used to store guaranteed collections and to generate block payloads.
type CollectionPool struct {
	*Mempool
}

// NewCollectionPool creates a new memory pool for guaranteed collections.
func NewCollectionPool() (*CollectionPool, error) {
	inner, err := NewMempool()
	if err != nil {
		return nil, err
	}

	m := &CollectionPool{inner}

	return m, nil
}

// Add adds a guaranteed collection to the mempool.
func (m *CollectionPool) Add(coll *collection.GuaranteedCollection) error {
	return m.Mempool.Add(coll)
}

// Get returns the given collection from the pool.
func (m *CollectionPool) Get(hash crypto.Hash) (*collection.GuaranteedCollection, error) {
	item, err := m.Mempool.Get(hash)
	if err != nil {
		return nil, err
	}

	coll, ok := item.(*collection.GuaranteedCollection)
	if !ok {
		return nil, fmt.Errorf("unable to convert item to guaranteed collection")
	}

	return coll, nil
}

// All returns all collections from the pool.
func (m *CollectionPool) All() []*collection.GuaranteedCollection {
	items := m.Mempool.All()

	colls := make([]*collection.GuaranteedCollection, len(items))
	for i, item := range items {
		colls[i] = item.(*collection.GuaranteedCollection)
	}

	return colls
}
