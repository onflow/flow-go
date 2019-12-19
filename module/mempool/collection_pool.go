// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// CollectionPool implements the collections memory pool of the consensus nodes,
// used to store guaranteed collections and to generate block payloads.
type CollectionPool struct {
	*mempool
}

// NewCollectionPool creates a new memory pool for guaranteed collections.
func NewCollectionPool() (*CollectionPool, error) {
	m := &CollectionPool{
		mempool: newMempool(),
	}

	return m, nil
}

// Add adds a guaranteed collection to the mempool.
func (m *CollectionPool) Add(coll *flow.GuaranteedCollection) error {
	return m.mempool.Add(coll)
}

// Get returns the given collection from the pool.
func (m *CollectionPool) Get(hash crypto.Hash) (*flow.GuaranteedCollection, error) {
	item, err := m.mempool.Get(hash)
	if err != nil {
		return nil, err
	}

	coll, ok := item.(*flow.GuaranteedCollection)
	if !ok {
		return nil, fmt.Errorf("unable to convert item to guaranteed collection")
	}

	return coll, nil
}

// All returns all collections from the pool.
func (m *CollectionPool) All() []*flow.GuaranteedCollection {
	items := m.mempool.All()

	colls := make([]*flow.GuaranteedCollection, len(items))
	for i, item := range items {
		colls[i] = item.(*flow.GuaranteedCollection)
	}

	return colls
}
