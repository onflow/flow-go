package stdmap

import (
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
)

// PendingCollections implements a mempool storing collections.
type PendingCollections struct {
	*Backend
	counterMU sync.Mutex // provides atomic updates for the counter
	counter   uint       // keeps number of added items to the mempool
}

// NewCollections creates a new memory pool for pending collection.
func NewPendingCollections(limit uint) (*PendingCollections, error) {
	p := &PendingCollections{
		counter: 0,
		Backend: NewBackend(WithLimit(limit), WithEject(ejectOldestPendingCollection)),
	}

	return p, nil
}

// Add adds a pending collection to the mempool.
func (p *PendingCollections) Add(pcoll *verification.PendingCollection) bool {
	p.counterMU.Lock()
	defer p.counterMU.Unlock()

	p.counter += 1
	pcoll.Counter = p.counter
	return p.Backend.Add(pcoll)
}

// Rem removes a pending collection by ID from memory
func (p *PendingCollections) Rem(collID flow.Identifier) bool {
	return p.Backend.Rem(collID)
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

// ejectOldestPendingCollection is the ejection function for pending collections, it finds and returns
// the entry with the largest counter value
func ejectOldestPendingCollection(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
	var oldestEntityID flow.Identifier
	var oldestEntity flow.Entity

	var maxCounter uint = 0
	for entityID, entity := range entities {
		pc := entity.(*verification.PendingCollection)
		if pc.Counter > maxCounter {
			maxCounter = pc.Counter
			oldestEntity = entity
			oldestEntityID = entityID
		}
	}
	return oldestEntityID, oldestEntity
}
