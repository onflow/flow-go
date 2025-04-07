package stdmap

import (
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/onflow/flow-go/utils/rand"
)

// this is the threshold for how much over the guaranteed capacity the
// collection should be before performing a batch ejection
const overCapacityThreshold = 128

// BatchEjectFunc implements an ejection policy to remove elements when the mempool
// exceeds its specified capacity. A custom ejection policy can be injected
// into the memory pool upon creation to change the strategy of eviction.
// The ejection policy is executed from within the thread that serves the
// mempool. Implementations should adhere to the following convention:
//   - The ejector function has the freedom to eject _multiple_ elements.
//   - In a single `eject` call, it must eject as many elements to statistically
//     keep the mempool size within the desired limit.
//   - The ejector _might_ (for performance reasons) retain more elements in the
//     mempool than the targeted capacity.
//   - The ejector _must_ notify the `Backend.ejectionCallbacks` for _each_
//     element it removes from the mempool.
//   - Implementations do _not_ need to be concurrency safe. The Backend handles
//     concurrency (specifically, it locks the mempool during ejection).
//   - The implementation should be non-blocking (though, it is allowed to
//     take a bit of time; the mempool will just be locked during this time).
type BatchEjectFunc[K comparable, V any] func(b *Backend[K, V]) (bool, error)
type EjectFunc[K comparable, V any] func(b *Backend[K, V]) (K, V, bool)

// EjectRandomFast checks if the map size is beyond the
// threshold size, and will iterate through them and eject unneeded
// entries if that is the case.  Return values are unused
func EjectRandomFast[K comparable, V any](b *Backend[K, V]) (bool, error) {
	currentSize := b.mutableBackData.Size()

	if b.guaranteedCapacity >= currentSize {
		return false, nil
	}
	// At this point, we know that currentSize > b.guaranteedCapacity. As
	// currentSize fits into an int, b.guaranteedCapacity must also fit.
	overcapacity := currentSize - b.guaranteedCapacity
	if overcapacity <= overCapacityThreshold {
		return false, nil
	}

	// Randomly select indices of elements to remove:
	mapIndices := make([]int, 0, overcapacity)
	for i := overcapacity; i > 0; i-- {
		rand, err := rand.Uintn(currentSize)
		if err != nil {
			return false, fmt.Errorf("random generation failed: %w", err)
		}
		mapIndices = append(mapIndices, int(rand))
	}
	sort.Ints(mapIndices) // inplace

	// Now, mapIndices is a sequentially sorted list of indices to remove.
	// Remove them in a loop. Repeated indices are idempotent (subsequent
	// ejection calls will make up for it).
	idx := 0                     // index into mapIndices
	next2Remove := mapIndices[0] // index of the element to be removed next
	i := 0                       // index into the entities map
	for key, value := range b.mutableBackData.All() {
		if i == next2Remove {
			b.mutableBackData.Remove(key) // remove entity
			for _, callback := range b.ejectionCallbacks {
				callback(value) // notify callback
			}

			idx++

			// There is a (1 in b.guaranteedCapacity) chance that the
			// next value in mapIndices is a duplicate. If a duplicate is
			// found, skip it by incrementing 'idx'
			for ; idx < int(overcapacity) && next2Remove == mapIndices[idx]; idx++ {
			}

			if idx == int(overcapacity) {
				return true, nil
			}
			next2Remove = mapIndices[idx]
		}
		i++
	}
	return true, nil
}

// EjectPanic simply panics, crashing the program. Useful when cache is not expected
// to grow beyond certain limits, but ejecting is not applicable
func EjectPanic[K comparable, V any](b *Backend[K, V]) (K, V, bool) {
	panic("unexpected: mempool size over the limit")
}

// LRUEjector provides a swift FIFO ejection functionality
type LRUEjector[K comparable] struct {
	sync.Mutex
	table  map[K]uint64 // keeps sequence number of values it tracks
	seqNum uint64       // keeps the most recent sequence number
}

func NewLRUEjector[K comparable]() *LRUEjector[K] {
	return &LRUEjector[K]{
		table:  make(map[K]uint64),
		seqNum: 0,
	}
}

// Track should be called every time a new entity is added to the mempool.
// It tracks the entity for later ejection.
func (q *LRUEjector[K]) Track(key K) {
	q.Lock()
	defer q.Unlock()

	if _, ok := q.table[key]; ok {
		// skips adding duplicate item
		return
	}

	// TODO current table structure provides O(1) track and untrack features
	// however, the Eject functionality is asymptotically O(n).
	// With proper resource cleanups by the mempools, the Eject is supposed
	// as a very infrequent operation. However, further optimizations on
	// Eject efficiency is needed.
	q.table[key] = q.seqNum
	q.seqNum++
}

// Untrack simply removes the tracker of the ejector off the entityID
func (q *LRUEjector[K]) Untrack(key K) {
	q.Lock()
	defer q.Unlock()

	delete(q.table, key)
}

// Eject implements EjectFunc for LRUEjector. It finds the value with the lowest sequence number (i.e.,
// the oldest entity). It also untracks. This is using a linear search.
func Eject[K comparable, V any](q *LRUEjector[K], b *Backend[K, V]) K {
	q.Lock()
	defer q.Unlock()

	// finds the oldest entity
	oldestSQ := uint64(math.MaxUint64)
	var oldestID K
	for _, id := range b.mutableBackData.Keys() {
		if sq, ok := q.table[id]; ok {
			if sq < oldestSQ {
				oldestID = id
				oldestSQ = sq
			}
		}
	}

	// untracks the oldest id as it is supposed to be ejected
	delete(q.table, oldestID)

	return oldestID
}
