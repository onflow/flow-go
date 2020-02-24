// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Receipts implements the execution receipts memory pool of the consensus node,
// used to store execution receipts and to generate block seals.
type Receipts struct {
	*Backend
	byBlock map[flow.Identifier](map[flow.Identifier]struct{})
}

// NewReceipts creates a new memory pool for execution receipts.
func NewReceipts() (*Receipts, error) {
	r := &Receipts{
		Backend: NewBackend(),
		byBlock: make(map[flow.Identifier](map[flow.Identifier]struct{})),
	}

	return r, nil
}

// Add adds an execution receipt to the mempool.
func (r *Receipts) Add(receipt *flow.ExecutionReceipt) error {
	err := r.Backend.Add(receipt)
	if err != nil {
		return err
	}
	blockID := receipt.ExecutionResult.BlockID
	forBlock, ok := r.byBlock[blockID]
	if !ok {
		forBlock = make(map[flow.Identifier]struct{})
		r.byBlock[blockID] = forBlock
	}
	forBlock[receipt.ID()] = struct{}{}
	return nil
}

// Rem will remove a receipt by ID.
func (r *Receipts) Rem(receiptID flow.Identifier) bool {
	entity, err := r.Backend.ByID(receiptID)
	if err != nil {
		return false
	}
	_ = r.Backend.Rem(receiptID)
	receipt := entity.(*flow.ExecutionReceipt)
	blockID := receipt.ExecutionResult.BlockID
	forBlock := r.byBlock[blockID]
	delete(forBlock, receiptID)
	if len(forBlock) > 0 {
		return true
	}
	delete(r.byBlock, blockID)
	return true
}

// ByBlockID returns an execution receipt by approval ID.
func (r *Receipts) ByBlockID(blockID flow.Identifier) []*flow.ExecutionReceipt {
	byBlock, ok := r.byBlock[blockID]
	if !ok {
		return nil
	}
	receipts := make([]*flow.ExecutionReceipt, 0, len(byBlock))
	for receiptID := range byBlock {
		entity, _ := r.Backend.ByID(receiptID)
		receipts = append(receipts, entity.(*flow.ExecutionReceipt))
	}
	return receipts
}

// DropForBlock drops all execution receipts for the given block.
func (r *Receipts) DropForBlock(blockID flow.Identifier) {
	byBlock, ok := r.byBlock[blockID]
	if !ok {
		return
	}
	for receiptID := range byBlock {
		_ = r.Backend.Rem(receiptID)
	}
	delete(r.byBlock, blockID)
}

// All will return all execution receipts in the memory pool.
func (r *Receipts) All() []*flow.ExecutionReceipt {
	entities := r.Backend.All()
	receipts := make([]*flow.ExecutionReceipt, 0, len(entities))
	for _, entity := range entities {
		receipts = append(receipts, entity.(*flow.ExecutionReceipt))
	}
	return receipts
}
