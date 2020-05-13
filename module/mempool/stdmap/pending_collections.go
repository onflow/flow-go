package stdmap

import (
	"math"
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
)

// TODO consolidate with PendingReceipts to preserve DRY
// https://github.com/dapperlabs/flow-go/issues/3690

// PendingCollections implements a mempool storing collections.
type PendingCollections struct {
	*Backend
	counterMU sync.Mutex // provides atomic updates for the counter
	counter   uint64     // keeps number of added items to the mempool
}

// NewCollections creates a new memory pool for pending collection.
func NewPendingCollections(limit uint) (*PendingCollections, error) {
	p := &PendingCollections{
		counter: 0,
		Backend: NewBackend(WithLimit(limit), WithEject(pendingCollectionLRUEject)),
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

// pendingCollectionLRUEject is the ejection function for pending collections, it finds and returns
// the entry with the largest counter value, i.e., the least recently added
func pendingCollectionLRUEject(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
	var oldestEntityID flow.Identifier
	var oldestEntity flow.Entity

	var minCounter uint64 = math.MaxUint64
	for entityID, entity := range entities {
		pc := entity.(*verification.PendingCollection)
		if pc.Counter < minCounter {
			minCounter = pc.Counter
			oldestEntity = entity
			oldestEntityID = entityID
		}
	}
	return oldestEntityID, oldestEntity
}
