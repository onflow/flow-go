// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// CollectionGuaranteePool implements the collections memory pool of the consensus nodes,
// used to store guaranteed collections and to generate block payloads.
type CollectionGuaranteePool struct {
	*mempool
}

// NewGuaranteePool creates a new memory pool for guaranteed collections.
func NewGuaranteePool() (*CollectionGuaranteePool, error) {
	m := &CollectionGuaranteePool{
		mempool: newMempool(),
	}

	return m, nil
}

// Add adds a guaranteed collection guarantee to the mempool.
func (m *CollectionGuaranteePool) Add(guarantee *flow.CollectionGuarantee) error {
	return m.mempool.Add(guarantee)
}

// Get returns the given collection guarantee from the pool.
func (m *CollectionGuaranteePool) Get(collID flow.Identifier) (*flow.CollectionGuarantee, error) {
	item, err := m.mempool.Get(collID)
	if err != nil {
		return nil, err
	}

	guarantee, ok := item.(*flow.CollectionGuarantee)
	if !ok {
		return nil, fmt.Errorf("unable to convert item to guaranteed collection")
	}

	return guarantee, nil
}

// All returns all collections from the pool.
func (m *CollectionGuaranteePool) All() []*flow.CollectionGuarantee {

	items := m.mempool.All()
	guarantees := make([]*flow.CollectionGuarantee, 0, len(items))
	for _, item := range items {
		guarantees = append(guarantees, item.(*flow.CollectionGuarantee))
	}

	return guarantees
}
