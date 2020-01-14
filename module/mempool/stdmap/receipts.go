// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Receipts implements the execution receipts memory pool of the consensus node,
// used to store execution receipts and to generate block seals.
type Receipts struct {
	*Backend
}

// NewReceipts creates a new memory pool for execution receipts.
func NewReceipts() (*Receipts, error) {
	r := &Receipts{
		Backend: NewBackend(),
	}

	return r, nil
}

// Add adds an execution receipt to the mempool.
func (r *Receipts) Add(receipt *flow.ExecutionReceipt) error {
	return r.Backend.Add(receipt)
}

// Get returns the execution receipt with the given ID from the mempool.
func (r *Receipts) Get(receiptID flow.Identifier) (*flow.ExecutionReceipt, error) {
	entity, err := r.Backend.Get(receiptID)
	if err != nil {
		return nil, err
	}
	receipt, ok := entity.(*flow.ExecutionReceipt)
	if !ok {
		panic(fmt.Sprintf("invalid entity in receipt pool (%T)", entity))
	}
	return receipt, nil
}

// All returns all execution receipts from the pool.
func (r *Receipts) All() []*flow.ExecutionReceipt {
	entities := r.Backend.All()
	receipts := make([]*flow.ExecutionReceipt, 0, len(entities))
	for _, entity := range entities {
		receipt, ok := entity.(*flow.ExecutionReceipt)
		if !ok {
			panic(fmt.Sprintf("invalid entity in receipt pool (%T)", entity))
		}
		receipts = append(receipts, receipt)
	}
	return receipts
}
