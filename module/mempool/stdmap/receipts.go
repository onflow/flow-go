// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
)

// Receipts implements the execution receipts memory pool of the consensus node,
// used to store execution receipts and to generate block seals.
type Receipts struct {
	*Backend
	byResult map[flow.Identifier]flow.Identifier
}

// NewReceipts creates a new memory pool for execution receipts.
func NewReceipts() (*Receipts, error) {
	r := &Receipts{
		Backend:  NewBackend(),
		byResult: make(map[flow.Identifier]flow.Identifier),
	}

	return r, nil
}

// Add adds an execution receipt to the mempool.
func (r *Receipts) Add(receipt *flow.ExecutionReceipt) error {
	err := r.Backend.Add(receipt)
	if err != nil {
		return err
	}
	r.byResult[receipt.ExecutionResult.ID()] = receipt.ID()
	return nil
}

// Rem removes a result approval from the mempool.
func (r *Receipts) Rem(receiptID flow.Identifier) bool {
	receipt, err := r.ByID(receiptID)
	if err != nil {
		return false
	}
	ok := r.Backend.Rem(receiptID)
	if !ok {
		return false
	}
	delete(r.byResult, receipt.ExecutionResult.ID())
	return true
}

// ByID returns the execution receipt with the given ID from the mempool.
func (r *Receipts) ByID(receiptID flow.Identifier) (*flow.ExecutionReceipt, error) {
	entity, err := r.Backend.ByID(receiptID)
	if err != nil {
		return nil, err
	}
	receipt, ok := entity.(*flow.ExecutionReceipt)
	if !ok {
		panic(fmt.Sprintf("invalid entity in receipt pool (%T)", entity))
	}
	return receipt, nil
}

// ByResultID returns an execution receipt by approval ID.
func (r *Receipts) ByResultID(resultID flow.Identifier) (*flow.ExecutionReceipt, error) {
	receiptID, ok := r.byResult[resultID]
	if !ok {
		return nil, mempool.ErrEntityNotFound
	}
	return r.ByID(receiptID)
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
