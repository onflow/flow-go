// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"math"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/stdmap/backdata"
	_ "github.com/onflow/flow-go/utils/binstat"
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
// This is using EjectRandomFast()
func NewBackend(options ...OptionFunc) *Backend {
	b := Backend{
		backData:           backdata.NewMapBackData(),
		guaranteedCapacity: uint(math.MaxUint32),
		batchEject:         EjectRandomFast,
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
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".r_lock.(Backend)Has")
	b.RLock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)Has")
	// defer binstat.Leave(bs2)
	defer b.RUnlock()
	has := b.backData.Has(entityID)
	return has
}

// Add adds the given item to the pool.
func (b *Backend) Add(entity flow.Entity) bool {
	// bs0 := binstat.EnterTime(binstat.BinStdmap + ".<<lock.(Backend)Add")
	entityID := entity.ID() // this expensive operation done OUTSIDE of lock :-)
	// binstat.Leave(bs0)

	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".w_lock.(Backend)Add")
	b.Lock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)Add")
	// defer binstat.Leave(bs2)
	defer b.Unlock()
	added := b.backData.Add(entityID, entity)
	b.reduce()
	return added
}

// Remove will remove the item with the given hash.
func (b *Backend) Remove(entityID flow.Identifier) bool {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".w_lock.(Backend)Remove")
	b.Lock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)Remove")
	// defer binstat.Leave(bs2)
	defer b.Unlock()
	_, removed := b.backData.Remove(entityID)
	return removed
}

// Adjust will adjust the value item using the given function if the given key can be found.
// Returns a bool which indicates whether the value was updated.
func (b *Backend) Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool) {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".w_lock.(Backend)Adjust")
	b.Lock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)Adjust")
	// defer binstat.Leave(bs2)
	defer b.Unlock()
	entity, wasUpdated := b.backData.Adjust(entityID, f)
	return entity, wasUpdated
}

// GetOrInit returns the given entity from the backdata. If the entity does not exist, it creates a new entity
// using the factory function and stores it in the backdata.
// Args:
// - entityID: the identifier of the entity to get.
// - init: the function that initializes the entity when it is not found.
// Returns:
//   - the entity.
//
// - a bool which indicates whether the entity was found (or created).
func (b *Backend) GetOrInit(entityID flow.Identifier, init func() flow.Entity) (flow.Entity, bool) {
	b.Lock()
	defer b.Unlock()

	return b.backData.GetOrInit(entityID, init)
}

// AdjustWithInit adjusts the entity using the given function if the given identifier can be found. When the
// entity is not found, it initializes the entity using the given init function and then applies the adjust function.
// Args:
// - entityID: the identifier of the entity to adjust.
// - adjust: the function that adjusts the entity.
// - init: the function that initializes the entity when it is not found.
// Returns:
//   - the adjusted entity.
//
// - a bool which indicates whether the entity was adjusted.
func (b *Backend) AdjustWithInit(entityID flow.Identifier, adjust func(flow.Entity) flow.Entity, init func() flow.Entity) (flow.Entity, bool) {
	b.Lock()
	defer b.Unlock()

	return b.backData.AdjustWithInit(entityID, adjust, init)
}

// ByID returns the given item from the pool.
func (b *Backend) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".r_lock.(Backend)ByID")
	b.RLock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)ByID")
	// defer binstat.Leave(bs2)
	defer b.RUnlock()
	entity, exists := b.backData.ByID(entityID)
	return entity, exists
}

// Run executes a function giving it exclusive access to the backdata
func (b *Backend) Run(f func(backdata mempool.BackData) error) error {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".w_lock.(Backend)Run")
	b.Lock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)Run")
	// defer binstat.Leave(bs2)
	defer b.Unlock()
	err := f(b.backData)
	b.reduce()
	return err
}

// Size will return the size of the backend.
func (b *Backend) Size() uint {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".r_lock.(Backend)Size")
	b.RLock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)Size")
	// defer binstat.Leave(bs2)
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
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".r_lock.(Backend)All")
	b.RLock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)All")
	// defer binstat.Leave(bs2)
	defer b.RUnlock()

	return b.backData.Entities()
}

// Clear removes all entities from the pool.
func (b *Backend) Clear() {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".w_lock.(Backend)Clear")
	b.Lock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)Clear")
	// defer binstat.Leave(bs2)
	defer b.Unlock()
	b.backData.Clear()
}

// RegisterEjectionCallbacks adds the provided OnEjection callbacks
func (b *Backend) RegisterEjectionCallbacks(callbacks ...mempool.OnEjection) {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".r_lock.(Backend)RegisterEjectionCallbacks")
	b.Lock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)RegisterEjectionCallbacks")
	// defer binstat.Leave(bs2)
	defer b.Unlock()
	b.ejectionCallbacks = append(b.ejectionCallbacks, callbacks...)
}

// reduce will reduce the size of the kept entities until we are within the
// configured memory pool size limit.
func (b *Backend) reduce() {
	// bs := binstat.EnterTime(binstat.BinStdmap + ".??lock.(Backend)reduce")
	// defer binstat.Leave(bs)

	// we keep reducing the cache size until we are at limit again
	// this was a loop, but the loop is now in EjectRandomFast()
	// the ejections are batched, so this call to eject() may not actually
	// do anything until the batch threshold is reached (currently 128)
	if b.backData.Size() > b.guaranteedCapacity {
		// get the key from the eject function
		// we don't do anything if there is an error
		if b.batchEject != nil {
			_, _ = b.batchEject(b)
		} else {
			_, _, _ = b.eject(b)
		}
	}
}
