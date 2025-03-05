package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// Receipts implements the execution receipts memory pool of the consensus node,
// used to store execution receipts and to generate block seals.
type Receipts struct {
	*Backend[flow.Identifier, *flow.ExecutionReceipt]
}

// NewReceipts creates a new memory pool for execution receipts.
func NewReceipts(limit uint) (*Receipts, error) {
	// create the receipts memory pool with the lookup maps
	r := &Receipts{
		Backend: NewBackend(WithLimit[flow.Identifier, *flow.ExecutionReceipt](limit)),
	}

	return r, nil
}

// Add adds an execution receipt to the mempool.
func (r *Receipts) Add(receipt *flow.ExecutionReceipt) bool {
	added := r.Backend.Add(receipt.ID(), receipt)
	return added
}

// Remove will remove a receipt by ID.
func (r *Receipts) Remove(receiptID flow.Identifier) bool {
	removed := r.Backend.Remove(receiptID)
	return removed
}

// ByID will retrieve an approval by ID.
func (r *Receipts) ByID(receiptID flow.Identifier) (*flow.ExecutionReceipt, bool) {
	receipt, exists := r.Backend.ByID(receiptID)
	if !exists {
		return nil, false
	}
	return receipt, true
}

// All will return all execution receipts in the memory pool.
func (r *Receipts) All() []*flow.ExecutionReceipt {
	entities := r.Backend.All()
	receipts := make([]*flow.ExecutionReceipt, 0, len(entities))
	for _, receipt := range entities {
		receipts = append(receipts, receipt)
	}
	return receipts
}
