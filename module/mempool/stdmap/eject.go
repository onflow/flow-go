// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"math"
	"math/rand"
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

// EjectFunc is a function used to pick an entity to evict from the memory pool
// backend when it overflows its limit. A custom eject function can be injected
// into the memory pool upon creation, which allows us to hook into the eject
// to clean up auxiliary data and/or to change the strategy of eviction.
type EjectFunc func(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity)

// EjectFakeRandom relies on the random map iteration in Go to pick the entity we eject
// from the entity set. It picks the first entity upon iteration, thus being the fastest
// way to pick an entity to be evicted; at the same time, it conserves the random bias
// of the Go map iteration.
func EjectFakeRandom(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
	var entityID flow.Identifier
	var entity flow.Entity
	for entityID, entity = range entities {
		break
	}
	return entityID, entity
}

// EjectTrueRandom relies on a random generator to pick a random entity to eject from the
// entity set. It will, on average, iterate through half the entities of the set. However,
// it provides us with a truly evenly distributed random selection.
func EjectTrueRandom(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
	var entityID flow.Identifier
	var entity flow.Entity
	i := 0
	n := rand.Intn(len(entities))
	for entityID, entity = range entities {
		if i == n {
			break
		}
		i++
	}
	return entityID, entity
}

// EjectPanic simply panics, crashing the program. Useful when cache is not expected
// to grow beyond certain limits, but ejecting is not applicable
func EjectPanic(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
	panic("unexpected: mempool size over the limit")
}

// LRUEjector provides a swift FIFO ejection functionality
type LRUEjector struct {
	sync.Mutex
	table  map[flow.Identifier]uint64 // keeps sequence number of entities it tracks
	seqNum uint64                     // keeps the most recent sequence number
}

func NewLRUEjector() *LRUEjector {
	return &LRUEjector{
		table:  make(map[flow.Identifier]uint64),
		seqNum: 0,
	}
}

// Track should be called every time a new entity is added to the mempool.
// It tracks the entity for later ejection.
func (q *LRUEjector) Track(entityID flow.Identifier) {
	q.Lock()
	defer q.Unlock()

	if _, ok := q.table[entityID]; ok {
		// skips adding duplicate item
		return
	}

	// TODO current table structure provides O(1) track and untrack features
	// however, the Eject functionality is asymptotically O(n).
	// With proper resource cleanups by the mempools, the Eject is supposed
	// as a very less frequent operation. However, further optimizations on
	// Eject efficiency is needed.
	q.table[entityID] = q.seqNum
	q.seqNum++
}

// Untrack simply removes the tracker of the ejector off the entityID
func (q *LRUEjector) Untrack(entityID flow.Identifier) {
	q.Lock()
	defer q.Unlock()

	delete(q.table, entityID)
}

// Eject implements EjectFunc for LRUEjector. It finds the entity with the lowest sequence number (i.e.,
//the oldest entity). It also untracks
func (q *LRUEjector) Eject(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
	q.Lock()
	defer q.Unlock()

	// finds the oldest entity
	oldestSQ := uint64(math.MaxUint64)
	var oldestID flow.Identifier
	for id := range entities {
		if sq, ok := q.table[id]; ok {
			if sq < oldestSQ {
				oldestID = id
				oldestSQ = sq
			}
		}
	}

	oldestEntity, ok := entities[oldestID]

	if !ok {
		oldestID, oldestEntity = EjectTrueRandom(entities)
	}

	// untracks the oldest id as it is supposed to be ejected
	delete(q.table, oldestID)

	return oldestID, oldestEntity
}

// SizeEjector is a wrapper around EjectTrueRandom that can be used to decrement
// an external size variable everytime an item is removed from the mempool.
// WARNING: requires external means for concurrency safety
// As SizeEjector writes to the externally-provided size variable, SizeEjector
// itself cannot provide concurrency safety for this variable. Instead, the
// concurrency safety must be implemented through the means of the Mempool which
// is using the ejector.
type SizeEjector struct {
	size *uint
}

// NewSizeEjector returns a SizeEjector linked to the provided size variable.
// WARNING: requires external means for concurrency safety
// As SizeEjector writes to the externally-provided size variable, SizeEjector
// itself cannot provide concurrency safety for this variable. Instead, the
// concurrency safety must be implemented through the means of the Mempool which
// is using the ejector.
func NewSizeEjector(size *uint) *SizeEjector {
	return &SizeEjector{
		size: size,
	}
}

// Eject calls EjectTrueRandom and decrements the size variable if an item was
// returned by EjectTrueRandom.
func (q *SizeEjector) Eject(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
	id, entity := EjectTrueRandom(entities)

	if _, ok := entities[id]; ok {
		*q.size--
	}

	return id, entity
}
