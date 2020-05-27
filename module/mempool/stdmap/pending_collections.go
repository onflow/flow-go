package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
)

// TODO consolidate with PendingReceipts to preserve DRY
// https://github.com/dapperlabs/flow-go/issues/3690

// PendingCollections implements a mempool storing collections.
type PendingCollections struct {
	*Backend
	qe *QueueEjector
}

// NewCollections creates a new memory pool for pending collection.
func NewPendingCollections(limit uint, collector module.MempoolMetrics) (*PendingCollections, error) {
	qe := NewQueueEjector(limit + 1)
	p := &PendingCollections{
		qe:      qe,
		Backend: NewBackend(WithLimit(limit), WithEject(qe.Eject)),
	}

	// registers size method of backend for metrics
	err := collector.Register(metrics.ResourcePendingCollection, p.Backend.Size)
	if err != nil {
		return nil, fmt.Errorf("could not register backend metric: %w", err)
	}

	return p, nil
}

// Add adds a pending collection to the mempool.
func (p *PendingCollections) Add(pcoll *verification.PendingCollection) bool {
	ok := p.Backend.Add(pcoll)
	if ok {
		p.qe.Push(pcoll.ID())
	}
	return ok
}

// Rem removes a pending collection by ID from memory
func (p *PendingCollections) Rem(collID flow.Identifier) bool {
	ok := p.Backend.Rem(collID)
	return ok
}

// ByID returns the pending collection with the given ID from the mempool.
func (p *PendingCollections) ByID(collID flow.Identifier) (*verification.PendingCollection, bool) {
	entity, exists := p.Backend.ByID(collID)
	if exists {
		return nil, false
	}
	pcoll := entity.(*verification.PendingCollection)
	return pcoll, true
}

// All returns all pending collections from the mempool.
func (p *PendingCollections) All() []*verification.PendingCollection {
	entities := p.Backend.All()
	pcolls := make([]*verification.PendingCollection, 0, len(entities))
	for _, entity := range entities {
		pcolls = append(pcolls, entity.(*verification.PendingCollection))
	}
	return pcolls
}
