// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"math"
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
)

// TODO consolidate with PendingCollections to preserve DRY
// https://github.com/dapperlabs/flow-go/issues/3690
// PendingReceipts implements the execution receipts memory pool of the consensus node,
// used to store execution receipts and to generate block seals.
type PendingReceipts struct {
	*Backend
	counterMU sync.Mutex // provides atomic updates for the counter
	counter   uint64     // keeps number of added items to the mempool
}

// NewReceipts creates a new memory pool for execution receipts.
func NewPendingReceipts(limit uint) (*PendingReceipts, error) {
	// create the receipts memory pool with the lookup maps
	r := &PendingReceipts{
		counter: 0,
		Backend: NewBackend(WithLimit(limit), WithEject(ejectOldestPendingReceipt)),
	}
	return r, nil
}

// Add adds a pending execution receipt to the mempool.
func (p *PendingReceipts) Add(preceipt *verification.PendingReceipt) bool {
	p.counterMU.Lock()
	defer p.counterMU.Unlock()

	p.counter += 1
	preceipt.Counter = p.counter
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

// ejectOldestPendingReceipt is the ejection function for pending receipts, it finds and returns
// the entry with the largest counter value
func ejectOldestPendingReceipt(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
	var oldestEntityID flow.Identifier
	var oldestEntity flow.Entity

	var minCounter uint64 = math.MaxUint64
	for entityID, entity := range entities {
		pc := entity.(*verification.PendingReceipt)
		if pc.Counter < minCounter {
			minCounter = pc.Counter
			oldestEntity = entity
			oldestEntityID = entityID
		}
	}
	return oldestEntityID, oldestEntity
}
