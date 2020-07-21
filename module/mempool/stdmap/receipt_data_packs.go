// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
)

// ReceiptDataPacks implements the ReceiptsDataPack mempool.
type ReceiptDataPacks struct {
	*Backend
	qe *QueueEjector
}

// NewReceipts creates a new memory pool for execution receipts.
func NewReceiptDataPacks(limit uint) (*ReceiptDataPacks, error) {
	// create the receipts memory pool with the lookup maps
	qe := NewQueueEjector(limit + 1)
	r := &ReceiptDataPacks{
		qe:      qe,
		Backend: NewBackend(WithLimit(limit), WithEject(qe.Eject)),
	}

	return r, nil
}

// Add will add the given ReceiptDataPack to the memory pool. It will return
// false if it was already in the mempool.
func (r *ReceiptDataPacks) Add(rdp *verification.ReceiptDataPack) bool {
	ok := r.Backend.Add(rdp)
	if ok {
		r.qe.Push(rdp.ID())
	}
	return ok
}

// Get returns the ReceiptDataPack and true, if the ReceiptDataPack is in the
// mempool. Otherwise, it returns nil and false.
func (r *ReceiptDataPacks) Get(rdpID flow.Identifier) (*verification.ReceiptDataPack, bool) {
	entity, ok := r.Backend.ByID(rdpID)
	if !ok {
		return nil, false
	}

	pr, ok := entity.(*verification.ReceiptDataPack)
	if !ok {
		return nil, false
	}

	return pr, true
}

// Rem removes a ReceiptDataPack by ID.
func (r *ReceiptDataPacks) Rem(rdpID flow.Identifier) bool {
	ok := r.Backend.Rem(rdpID)
	return ok
}

// All will return all ReceiptDataPacks in the mempool.
func (r *ReceiptDataPacks) All() []*verification.ReceiptDataPack {
	entities := r.Backend.All()
	receipts := make([]*verification.ReceiptDataPack, 0, len(entities))
	for _, entity := range entities {
		receipts = append(receipts, entity.(*verification.ReceiptDataPack))
	}
	return receipts
}

// Size returns total number ReceiptDataPacks in mempool
func (r *ReceiptDataPacks) Size() uint {
	return r.Backend.Size()
}
