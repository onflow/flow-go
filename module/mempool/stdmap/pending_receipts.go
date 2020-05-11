// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
)

// PendingReceipts implements the execution receipts memory pool of the consensus node,
// used to store execution receipts and to generate block seals.
type PendingReceipts struct {
	sync.Mutex
	*Backend
}

// NewReceipts creates a new memory pool for execution receipts.
func NewPendingReceipts(limit uint) (*PendingReceipts, error) {
	// create the receipts memory pool with the lookup maps
	r := &PendingReceipts{
		Backend: NewBackend(WithLimit(limit)),
	}
	return r, nil
}

// Add adds a pending execution receipt to the mempool.
func (p *PendingReceipts) Add(preceipt *verification.PendingReceipt) bool {
	return p.Backend.Add(preceipt)
}

// Rem will remove a pending receipt by ID.
func (p *PendingReceipts) Rem(preceiptID flow.Identifier) bool {
	return p.Backend.Rem(preceiptID)
}

// All will return all pending execution receipts in the memory pool.
func (p *PendingReceipts) All() []*verification.PendingReceipt {
	entities := p.Backend.All()
	receipts := make([]*verification.PendingReceipt, 0, len(entities))
	for _, entity := range entities {
		receipts = append(receipts, entity.(*verification.PendingReceipt))
	}
	return receipts
}
