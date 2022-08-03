package common

import (
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

// IMPORTANT:  TrieQueue is in the wal package to prevent it
// from being used for other purposes and getting modified
// (to prevent introducing side-effects to checkpointing).

// TrieQueue is a fix-sized FIFO queue of MTrie.
// It is only used by Compactor for checkpointing, and
// it is intentionally not threadsafe given its limited use case.
// It is not a general purpose queue to avoid incurring overhead
// for features not needed for its limited use case.
type TrieQueue struct {
	ts       []*trie.MTrie
	capacity int
	tail     int // element index to write to
	count    int // number of elements (count <= capacity)
}

// NewTrieQueue returns a new TrieQueue with given capacity.
func NewTrieQueue(capacity uint) *TrieQueue {
	return &TrieQueue{
		ts:       make([]*trie.MTrie, capacity),
		capacity: int(capacity),
	}
}

// NewTrieQueueWithValues returns a new TrieQueue with given capacity and initial values.
func NewTrieQueueWithValues(capacity uint, tries []*trie.MTrie) *TrieQueue {
	q := NewTrieQueue(capacity)

	start := 0
	if len(tries) > q.capacity {
		start = len(tries) - q.capacity
	}
	n := copy(q.ts, tries[start:])
	q.count = n
	q.tail = q.count % q.capacity

	return q
}

// Push pushes trie to queue.  If queue is full, it overwrites the oldest element.
func (q *TrieQueue) Push(t *trie.MTrie) {
	q.ts[q.tail] = t
	q.tail = (q.tail + 1) % q.capacity
	if !q.isFull() {
		q.count++
	}
}

// Tries returns elements in queue, starting from the oldest element
// to the newest element.
func (q *TrieQueue) Tries() []*trie.MTrie {
	if q.count == 0 {
		return nil
	}

	tries := make([]*trie.MTrie, q.count)

	if q.tail >= q.count { // Data isn't wrapped around the slice.
		head := q.tail - q.count
		copy(tries, q.ts[head:q.tail])
	} else { // q.tail < q.count, data is wrapped around the slice.
		head := q.capacity - q.count + q.tail
		n := copy(tries, q.ts[head:])
		copy(tries[n:], q.ts[:q.tail])
	}

	return tries
}

// Count returns element count.
func (q *TrieQueue) Count() int {
	return q.count
}

func (q *TrieQueue) isFull() bool {
	return q.count == q.capacity
}
