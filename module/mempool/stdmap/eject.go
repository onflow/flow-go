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
type EjectFunc func(b *Backend) (flow.Identifier, flow.Entity, bool)

// EjectFakeRandom relies on the random map iteration in Go to pick the entity we eject
// from the entity set. It picks the first entity upon iteration, thus being the fastest
// way to pick an entity to be evicted; at the same time, it conserves the random bias
// of the Go map iteration.
func EjectFakeRandom(b *Backend) (flow.Identifier, flow.Entity, bool) {
	var entityID flow.Identifier
	var entity flow.Entity
	var bFound bool = false
	for entityID, entity = range b.entities {
		bFound = true
		break
	}
	return entityID, entity, bFound
}

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
// ideal size, and will iterate through them and eject unneeded
// entries if that is the case.
func EjectTrueRandomFast(b *Backend) (flow.Identifier, flow.Entity, bool) {

	// this is 64 for performance reasons - modulus 64 is a bit shift
	const batchSize = 64

	// 64 in batch plus a buffer of 64 to prevent boundary conditions from
	// rounding errors.  The buffer is also here because the max index
	// decreases as items are deleted
	const threshold = batchSize * 2

	var entities = b.entities

	var retval flow.Identifier 	// dummy value 

	// 'len' returns a uint.  This map is assumed to be < sizeof(uint)
	mapSize := len(entities)

	// this should never happen, and is just for a quick check
	if b.limit > uint(mapSize) {
		return retval, nil, false
	}

	if (uint(mapSize) - b.limit) <= threshold {
		// nothing to do, yet
		return retval, nil, false
	}

	// generate 64 random numbers (the additional 64 is so that we don't
	// go outside the boundary once the elements are ejected)

	var entityID flow.Identifier
	var entity flow.Entity

	// mapSize is at LEAST 128 over the desired size. We want to eject
	// up to 64 items (so index is not OOB), so remove 64 from map size
	maxInterval := (mapSize - batchSize) / batchSize

	// this array will store 64 indexes into the map
	var mapIndexes [batchSize]int64

	// starting point, create 64 random, sequentially increasing, values
	var index int64 = 0

	var i uint = 0
	for ; i < batchSize-1; i++ {
		// get a random number (zero-based) between 0 and maxInterval
		index += int64(rand.Intn(maxInterval - 1))

		mapIndexes[i] = index

		// increment so we're not looking at the same element if rnd is 0
		index++
	}

	// for the last item in 'batchSize', the random number should use the
	// remaining count as the parameter for the random value
	index += int64(rand.Intn(mapSize - 1 - int(index)))
	mapIndexes[i] = index

	// Now, mapIndexes has a sequentially sorted set of indexes to remove.
	// Remove them in a loop
	i = 0
	idx := 0 // index into mapIndexes
	for entityID, entity = range entities {
		if int64(i) == mapIndexes[idx] {
			// remove this entry here
			delete(entities, entityID)

			// notify callback
			for _, callback := range b.ejectionCallbacks {
				callback(entity)
			}

			// increment the index
			idx++
			if idx >= batchSize {
				break
			}
		}
		i++
	}
	return retval, nil, false
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
