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
func (p *PendingCollections) Add(pcoll *verification.PendingCollection) error {
	return p.Backend.Add(pcoll)
}

// Rem removes a pending collection by ID from memory
func (p *PendingCollections) Rem(pcollID flow.Identifier) bool {
	return p.Backend.Rem(pcollID)
}

// ByID returns the pending collection with the given ID from the mempool.
func (p *PendingCollections) ByID(pcollID flow.Identifier) (*verification.PendingCollection, error) {
	entity, err := p.Backend.ByID(pcollID)
	if err != nil {
		return nil, err
	}
	pcoll, ok := entity.(*verification.PendingCollection)
	if !ok {
		panic(fmt.Sprintf("invalid entity in pending collection pool (%T)", entity))
	}
	return pcoll, nil
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
