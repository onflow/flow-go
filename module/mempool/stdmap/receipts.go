// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Receipts implements the execution receipts memory pool of the consensus node,
// used to store execution receipts and to generate block seals.
type Receipts struct {
	*Backend
}

// NewReceipts creates a new memory pool for execution receipts.
func NewReceipts(limit uint) (*Receipts, error) {

	// create the receipts memory pool with the lookup maps
	r := &Receipts{
		Backend: NewBackend(WithLimit(limit)),
	}

	return r, nil
}

// Add adds an execution receipt to the mempool.
func (r *Receipts) Add(receipt *flow.ExecutionReceipt) bool {
	added := r.Backend.Add(receipt)
	if !added {
		return false
	}
	r.register(receipt)
	return true
}

// Rem will remove a receipt by ID.
func (r *Receipts) Rem(receiptID flow.Identifier) bool {
	entity, exists := r.Backend.ByID(receiptID)
	if !exists {
		return false
	}
	_ = r.Backend.Rem(receiptID)
	receipt := entity.(*flow.ExecutionReceipt)
	r.cleanup(receiptID, receipt)
	return true
}

// ByID will retrieve an approval by ID.
func (r *Receipts) ByID(receiptID flow.Identifier) (*flow.ExecutionReceipt, bool) {
	entity, exists := r.Backend.ByID(receiptID)
	if !exists {
		return nil, false
	}
	receipt := entity.(*flow.ExecutionReceipt)
	return receipt, true
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

// ByID will retrieve an approval by ID.
func (r *Receipts) ByID(receiptID flow.Identifier) (*flow.ExecutionReceipt, bool) {
	entity, exists := r.Backend.ByID(receiptID)
	if !exists {
		return nil, false
	}
	receipt := entity.(*flow.ExecutionReceipt)
	return receipt, true
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

// DropForBlock drops all execution receipts for the given block.
func (r *Receipts) DropForBlock(blockID flow.Identifier) []flow.Identifier {
	var receiptIDs []flow.Identifier
	for _, receipt := range r.All() {
		if receipt.ExecutionResult.BlockID == blockID {
			_ = r.Rem(receipt.ID())
			receiptIDs = append(receiptIDs, receipt.ID())
		}
	}
	return receiptIDs
}
