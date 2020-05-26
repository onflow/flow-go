// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Receipts implements the execution receipts memory pool of the consensus node,
// used to store execution receipts and to generate block seals.
type Receipts struct {
	*Backend
	sizeMeter func(uint) // keeps track of size variations of memory pool
}

type ReceiptsOpts func(*Receipts)

func WithSizeMeterReceipts(f func(uint)) ReceiptsOpts {
	return func(p *Receipts) {
		p.sizeMeter = f
	}
}

// NewReceipts creates a new memory pool for execution receipts.
func NewReceipts(limit uint, opts ...ReceiptsOpts) (*Receipts, error) {

	// create the receipts memory pool with the lookup maps
	r := &Receipts{
		Backend:   NewBackend(WithLimit(limit)),
		sizeMeter: nil,
	}

	for _, apply := range opts {
		apply(r)
	}

	return r, nil
}

// Add adds an execution receipt to the mempool.
func (r *Receipts) Add(receipt *flow.ExecutionReceipt) bool {
	added := r.Backend.Add(receipt)

	// tracks size update
	if r.sizeMeter != nil {
		r.sizeMeter(r.Backend.Size())
	}

	return added
}

// Rem will remove a receipt by ID.
func (r *Receipts) Rem(receiptID flow.Identifier) bool {
	removed := r.Backend.Rem(receiptID)

	// tracks size update
	if r.sizeMeter != nil {
		r.sizeMeter(r.Backend.Size())
	}

	return removed
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
