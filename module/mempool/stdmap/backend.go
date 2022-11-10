// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"math"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/stdmap/backdata"
)

// Backend provides synchronized access to a backdata
type Backend struct {
	sync.RWMutex
	backData           mempool.BackData
	guaranteedCapacity uint
	batchEject         BatchEjectFunc
	eject              EjectFunc
	ejectionCallbacks  []mempool.OnEjection
}

// NewBackend creates a new memory pool backend.
// This is using EjectTrueRandomFast()
func NewBackend(options ...OptionFunc) *Backend {
	b := Backend{
		backData:           backdata.NewMapBackData(),
		guaranteedCapacity: uint(math.MaxUint32),
		batchEject:         EjectTrueRandomFast,
		eject:              nil,
		ejectionCallbacks:  nil,
	}
	for _, option := range options {
		option(&b)
	}
	return &b
}

// Has checks if we already contain the item with the given hash.
func (b *Backend) Has(entityID flow.Identifier) bool {
	b.RLock()

	defer b.RUnlock()
	has := b.backData.Has(entityID)
	return has
}

// Add adds the given item to the pool.
func (b *Backend) Add(entity flow.Entity) bool {
	entityID := entity.ID() // this expensive operation done OUTSIDE of lock :-)

	b.Lock()

	defer b.Unlock()
	added := b.backData.Add(entityID, entity)
	b.reduce()
	return added
}

// Remove will remove the item with the given hash.
func (b *Backend) Remove(entityID flow.Identifier) bool {
	b.Lock()

	defer b.Unlock()
	_, removed := b.backData.Remove(entityID)
	return removed
}

// Adjust will adjust the value item using the given function if the given key can be found.
// Returns a bool which indicates whether the value was updated.
func (b *Backend) Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool) {
	b.Lock()

	defer b.Unlock()
	entity, wasUpdated := b.backData.Adjust(entityID, f)
	return entity, wasUpdated
}

// ByID returns the given item from the pool.
func (b *Backend) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	b.RLock()

	defer b.RUnlock()
	entity, exists := b.backData.ByID(entityID)
	return entity, exists
}

// Run executes a function giving it exclusive access to the backdata
func (b *Backend) Run(f func(backdata mempool.BackData) error) error {
	b.Lock()

	defer b.Unlock()
	err := f(b.backData)
	b.reduce()
	return err
}

// Size will return the size of the backend.
func (b *Backend) Size() uint {
	b.RLock()

	defer b.RUnlock()
	size := b.backData.Size()
	return size
}

// Limit returns the maximum number of items allowed in the backend.
func (b *Backend) Limit() uint {
	return b.guaranteedCapacity
}

// All returns all entities from the pool.
func (b *Backend) All() []flow.Entity {
	b.RLock()

	defer b.RUnlock()

	return b.backData.Entities()
}

// Clear removes all entities from the pool.
func (b *Backend) Clear() {
	b.Lock()

	defer b.Unlock()
	b.backData.Clear()
}

// Hash will use a merkle root hash to hash all items.
func (b *Backend) Hash() flow.Identifier {
	b.RLock()

	defer b.RUnlock()
	identifier := b.backData.Hash()
	return identifier
}

// RegisterEjectionCallbacks adds the provided OnEjection callbacks
func (b *Backend) RegisterEjectionCallbacks(callbacks ...mempool.OnEjection) {
	b.Lock()

	defer b.Unlock()
	b.ejectionCallbacks = append(b.ejectionCallbacks, callbacks...)
}

// reduce will reduce the size of the kept entities until we are within the
// configured memory pool size limit.
func (b *Backend) reduce() {

	// we keep reducing the cache size until we are at limit again
	// this was a loop, but the loop is now in EjectTrueRandomFast()
	// the ejections are batched, so this call to eject() may not actually
	// do anything until the batch threshold is reached (currently 128)
	if b.backData.Size() > b.guaranteedCapacity {
		// get the key from the eject function
		// we don't do anything if there is an error
		if b.batchEject != nil {
			_ = b.batchEject(b)
		} else {
			_, _, _ = b.eject(b)
		}
	}
}
