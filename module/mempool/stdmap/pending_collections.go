package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
)

// PendingCollections implements a mempool storing collections.
type PendingCollections struct {
	*Backend
}

// NewCollections creates a new memory pool for pending collection.
func NewPendingCollections(limit uint) (*PendingCollections, error) {
	p := &PendingCollections{
		Backend: NewBackend(WithLimit(limit)),
	}

	return p, nil
}

// Add adds a pending collection to the mempool.
func (p *PendingCollections) Add(pcoll *verification.PendingCollection) bool {
	return p.Backend.Add(pcoll)
}

// Rem removes a pending collection by ID from memory
func (p *PendingCollections) Rem(collID flow.Identifier) bool {
	return p.Backend.Rem(collID)
}

// Inc atomically increases the counter of pending collection by one and returns the updated collection
func (p *PendingCollections) Inc(collID flow.Identifier) (*verification.PendingCollection, error) {
	updated, ok := p.Backend.Adjust(collID, func(entity flow.Entity) flow.Entity {
		pc := entity.(*verification.PendingCollection)
		return &verification.PendingCollection{
			Collection: pc.Collection,
			OriginID:   pc.OriginID,
			Counter:    pc.Counter + 1,
		}
	})

	if !ok {
		return nil, fmt.Errorf("could not update pending collection in backend")
	}

	return updated.(*verification.PendingCollection), nil
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
