// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

// this is the threshold for how much over the guaranteed capacity the
// collection should be before performing a batch ejection
const overCapacityThreshold = 128

// [Batch]EjectFunc implements an ejection policy to remove elements when the mempool
// exceeds its specified capacity. A custom ejection policy can be injected
// into the memory pool upon creation to change the strategy of eviction.
// The ejection policy is executed from within the thread that serves the
// mempool. Implementations should adhere to the following convention:
//  * The ejector function has the freedom to eject _multiple_ elements.
//  * In a single `eject` call, it must eject as many elements to statistically
//    keep the mempool size within the desired limit.
//  * The ejector _might_ (for performance reasons) retain more elements in the
//    mempool than the targeted capacity.
//  * The ejector _must_ notify the `Backend.ejectionCallbacks` for _each_
//    element it removes from the mempool.
//  * Implementations do _not_ need to be concurrency safe. The Backend handles
//    concurrency (specifically, it locks the mempool during ejection).
//  * The implementation should be non-blocking (though, it is allowed to
//    take a bit of time; the mempool will just be locked during this time).
type BatchEjectFunc func(b *Backend) bool
type EjectFunc func(b *Backend) (flow.Identifier, flow.Entity, bool)

// EjectTrueRandom relies on a random generator to pick a random entity to eject from the
// entity set. It will, on average, iterate through half the entities of the set. However,
// it provides us with a truly evenly distributed random selection.
func EjectTrueRandom(b *Backend) (flow.Identifier, flow.Entity, bool) {
	var entityID flow.Identifier
	var entity flow.Entity
	var bFound bool = false
	i := 0
	n := rand.Intn(len(b.entities))
	for entityID, entity = range b.entities {
		if i == n {
			bFound = true
			break
		}
		i++
	}
	return entityID, entity, bFound
}

// EjectTrueRandomFast checks if the map size is beyond the
// threshold size, and will iterate through them and eject unneeded
// entries if that is the case.  Return values are unused
func EjectTrueRandomFast(b *Backend) bool {
	currentSize := len(b.entities)

	if b.guaranteedCapacity >= uint(currentSize) {
		return false
	}
	// At this point, we know that currentSize > b.guaranteedCapacity. As
	// currentSize fits into an int, b.guaranteedCapacity must also fit.
	overcapacity := currentSize - int(b.guaranteedCapacity)
	if overcapacity <= overCapacityThreshold {
		return false
	}

	// Randomly select indices of elements to remove:
	mapIndices := make([]int, 0, overcapacity)
	for i := overcapacity; i > 0; i-- {
		mapIndices = append(mapIndices, rand.Intn(currentSize))
	}
	sort.Ints(mapIndices) // inplace

	// Now, mapIndices is a sequentially sorted list of indices to remove.
	// Remove them in a loop. Repeated indices are idempotent (subsequent
	// ejection calls will make up for it).
	idx := 0                     // index into mapIndices
	next2Remove := mapIndices[0] // index of the element to be removed next
	i := 0                       // index into the entities map
	for entityID, entity := range b.entities {
		if i == next2Remove {
			delete(b.entities, entityID) // remove entity
			for _, callback := range b.ejectionCallbacks {
				callback(entity) // notify callback
			}

			idx++

			// There is a (1 in b.guaranteedCapacity) chance that the
			// next value in mapIndices is a duplicate. If a duplicate is
			// found, skip it by incrementing 'idx'
			for ; idx < overcapacity && next2Remove == mapIndices[idx]; idx++ {
			}

			if idx == overcapacity {
				return true
			}
			next2Remove = mapIndices[idx]
		}
		i++
	}
	return true
}

// EjectPanic simply panics, crashing the program. Useful when cache is not expected
// to grow beyond certain limits, but ejecting is not applicable
func EjectPanic(b *Backend) (flow.Identifier, flow.Entity, bool) {
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
	// as a very infrequent operation. However, further optimizations on
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
//the oldest entity). It also untracks.  This is using a linear search
func (q *LRUEjector) Eject(b *Backend) (flow.Identifier, flow.Entity, bool) {
	q.Lock()
	defer q.Unlock()

	// finds the oldest entity
	oldestSQ := uint64(math.MaxUint64)
	var oldestID flow.Identifier
	for id := range b.entities {
		if sq, ok := q.table[id]; ok {
			if sq < oldestSQ {
				oldestID = id
				oldestSQ = sq
			}
		}
	}

	// TODO:  don't do a lookup if it isn't necessary
	oldestEntity, ok := b.entities[oldestID]

	if !ok {
		oldestID, oldestEntity, ok = EjectTrueRandom(b)
	}

	// untracks the oldest id as it is supposed to be ejected
	delete(q.table, oldestID)

	return oldestID, oldestEntity, ok
}
