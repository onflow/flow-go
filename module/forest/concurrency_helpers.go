package forest

import (
	"sync"
)

/* ATTENTION: LevelledForest and its derived objects, such as the VertexIterator, are NOT Concurrency Safe. The
 * LevelledForest is a low-level library geared for performance. As locking is not needed in some application
 * scenarios (most notably the consensus EventHandler, which by design is single-threaded), concurrency handling
 * is delegated to the higher-level business logic using the LevelledForest.
 *
 * Here, we provide helper structs for higher-level business logic, to simplify their concurrency handling.
 */

// VertexIteratorConcurrencySafe wraps the Vertex Iterator to make it concurrency safe. Effectively,
// the behaviour is like iterating on a SNAPSHOT at the time of iterator construction.
// Under concurrent recalls, the iterator delivers each item once across all concurrent callers.
// Items are delivered in order and `NextVertex` establishes a 'synchronized before' relation as
// defined in the go memory model https://go.dev/ref/mem.
type VertexIteratorConcurrencySafe struct {
	unsafeIter VertexIterator
	mu         sync.RWMutex
}

func NewVertexIteratorConcurrencySafe(iter VertexIterator) *VertexIteratorConcurrencySafe {
	return &VertexIteratorConcurrencySafe{unsafeIter: iter}
}

// NextVertex returns the next Vertex or nil if there is none. A caller receiving a non-nil value
// are  'synchronized before' (see  https://go.dev/ref/mem) the receiver of the subsequent non-nil
// value. NextVertex() delivers each item once, following a fully sequential deterministic order,
// with results being distributed in order across all competing threads.
func (i *VertexIteratorConcurrencySafe) NextVertex() Vertex {
	i.mu.Lock() // must acquire write lock here, as wrapped `VertexIterator` changes its internal state
	defer i.mu.Unlock()
	return i.unsafeIter.NextVertex()
}

// HasNext returns true if and only if there is a next Vertex
func (i *VertexIteratorConcurrencySafe) HasNext() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.unsafeIter.HasNext()
}
