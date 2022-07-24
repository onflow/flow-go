package complete

import (
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

// CheckpointQueue is a fix-sized FIFO queue of MTrie.
// CheckpointQueue is intentionally not threadsafe given its limited use case.
// CheckpointQueue is not a general purpose queue to avoid incurring overhead
// for features not needed for its limited use case.
type CheckpointQueue struct {
	ts       []*trie.MTrie
	capacity int
	tail     int // element index to write to
	count    int // number of elements (count <= capacity)
}

// NewCheckpointQueue returns a new CheckpointQueue with given capacity.
func NewCheckpointQueue(capacity uint) *CheckpointQueue {
	return &CheckpointQueue{
		ts:       make([]*trie.MTrie, capacity),
		capacity: int(capacity),
	}
}

// NewCheckpointQueueWithValues returns a new CheckpointQueue with given capacity and initial values.
func NewCheckpointQueueWithValues(capacity uint, tries []*trie.MTrie) *CheckpointQueue {
	q := NewCheckpointQueue(capacity)

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
func (q *CheckpointQueue) Push(t *trie.MTrie) {
	q.ts[q.tail] = t
	q.tail = (q.tail + 1) % q.capacity
	if !q.isFull() {
		q.count++
	}
}

// Tries returns elements in queue, starting from the oldest element
// to the newest element.
func (q *CheckpointQueue) Tries() []*trie.MTrie {
	if q.count == 0 {
		return nil
	}

	tries := make([]*trie.MTrie, q.count)

	if q.isFull() {
		// If queue is full, tail points to the oldest element.
		head := q.tail
		n := copy(tries, q.ts[head:])
		copy(tries[n:], q.ts[:q.tail])
	} else {
		if q.tail >= q.count { // Data isn't wrapped around the slice.
			head := q.tail - q.count
			copy(tries, q.ts[head:q.tail])
		} else { // q.tail < q.count, data is wrapped around the slice.
			// This branch isn't used until CheckpointQueue supports Pop (removing oldest element).
			// At this time, there is no reason to implement Pop, so this branch is here to prevent future bug.
			head := q.capacity - q.count + q.tail
			n := copy(tries, q.ts[head:])
			copy(tries[n:], q.ts[:q.tail])
		}
	}

	return tries
}

// Count returns element count.
func (q *CheckpointQueue) Count() int {
	return q.count
}

func (q *CheckpointQueue) isFull() bool {
	return q.count == q.capacity
}
