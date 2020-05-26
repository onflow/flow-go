package stdmap

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
)

// TODO consolidate with PendingReceipts to preserve DRY
// https://github.com/dapperlabs/flow-go/issues/3690

type PendingCollectionsOpts func(*PendingCollections)

func WithSizeMeterPendingCollections(f func(uint)) PendingCollectionsOpts {
	return func(p *PendingCollections) {
		p.sizeMeter = f
	}
}

// PendingCollections implements a mempool storing collections.
type PendingCollections struct {
	*Backend
	qe        *QueueEjector
	sizeMeter func(uint) // keeps track of size variations of memory pool
}

// NewCollections creates a new memory pool for pending collection.
func NewPendingCollections(limit uint, opts ...PendingCollectionsOpts) (*PendingCollections, error) {
	qe := NewQueueEjector(limit + 1)
	p := &PendingCollections{
		qe:        qe,
		Backend:   NewBackend(WithLimit(limit), WithEject(qe.Eject)),
		sizeMeter: nil,
	}

	for _, apply := range opts {
		apply(p)
	}

	return p, nil
}

// Add adds a pending collection to the mempool.
func (p *PendingCollections) Add(pcoll *verification.PendingCollection) bool {
	ok := p.Backend.Add(pcoll)
	if ok {
		p.qe.Push(pcoll.ID())
	}

	if p.sizeMeter != nil {
		p.sizeMeter(p.Backend.Size())
	}

	return ok
}

// Rem removes a pending collection by ID from memory
func (p *PendingCollections) Rem(collID flow.Identifier) bool {
	ok := p.Backend.Rem(collID)
	if p.sizeMeter != nil {
		p.sizeMeter(p.Backend.Size())
	}

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
