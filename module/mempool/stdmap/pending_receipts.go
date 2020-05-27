// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
)

// TODO consolidate with PendingCollections to preserve DRY
// https://github.com/dapperlabs/flow-go/issues/3690
// PendingReceipts implements the execution receipts memory pool of the consensus node,
// used to store execution receipts and to generate block seals.
type PendingReceipts struct {
	*Backend
	qe *QueueEjector
}

// NewReceipts creates a new memory pool for execution receipts.
func NewPendingReceipts(limit uint, collector module.MempoolMetrics) (*PendingReceipts, error) {
	// create the receipts memory pool with the lookup maps
	qe := NewQueueEjector(limit + 1)
	r := &PendingReceipts{
		qe:      qe,
		Backend: NewBackend(WithLimit(limit), WithEject(qe.Eject)),
	}

	// registers size method of backend for metrics
	err := collector.Register(metrics.ResourcePendingReceipt, r.Backend.Size)
	if err != nil {
		return nil, fmt.Errorf("could not register backend metric: %w", err)
	}

	return r, nil
}

// Add adds a pending execution receipt to the mempool.
func (p *PendingReceipts) Add(preceipt *verification.PendingReceipt) bool {
	ok := p.Backend.Add(preceipt)
	if ok {
		p.qe.Push(preceipt.ID())
	}
	return ok
}

// Rem will remove a pending receipt by ID.
func (p *PendingReceipts) Rem(preceiptID flow.Identifier) bool {
	ok := p.Backend.Rem(preceiptID)
	return ok
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
