package wal

import (
	"github.com/onflow/flow-go/ledger/complete/payloadless"
)

// PayloadlessTrieQueue is a fix-sized FIFO queue of [payloadless.MTrie].
//
// It is the payloadless counterpart of [TrieQueue] and is intended for the
// same purpose: bookkeeping the rolling set of recent tries that a Compactor
// considers when emitting a checkpoint. Like [TrieQueue], it is intentionally
// not goroutine-safe — its sole expected caller is the single Compactor
// goroutine.
type PayloadlessTrieQueue struct {
	ts       []*payloadless.MTrie
	capacity int
	tail     int // element index to write to
	count    int // number of elements (count <= capacity)
}

// NewPayloadlessTrieQueue returns a new empty queue with the given capacity.
func NewPayloadlessTrieQueue(capacity uint) *PayloadlessTrieQueue {
	return &PayloadlessTrieQueue{
		ts:       make([]*payloadless.MTrie, capacity),
		capacity: int(capacity),
	}
}

// NewPayloadlessTrieQueueWithValues returns a new queue pre-populated with the
// given tries. If more than `capacity` tries are provided, only the
// `capacity` most recent ones are retained.
func NewPayloadlessTrieQueueWithValues(capacity uint, tries []*payloadless.MTrie) *PayloadlessTrieQueue {
	q := NewPayloadlessTrieQueue(capacity)

	start := 0
	if len(tries) > q.capacity {
		start = len(tries) - q.capacity
	}
	n := copy(q.ts, tries[start:])
	q.count = n
	q.tail = q.count % q.capacity
	return q
}

// Push appends a trie to the queue. When the queue is full, the oldest entry
// is overwritten in FIFO order.
func (q *PayloadlessTrieQueue) Push(t *payloadless.MTrie) {
	q.ts[q.tail] = t
	q.tail = (q.tail + 1) % q.capacity
	if !q.isFull() {
		q.count++
	}
}

// Tries returns the queued tries in FIFO order (oldest first). The returned
// slice is a fresh copy and is safe for the caller to retain.
func (q *PayloadlessTrieQueue) Tries() []*payloadless.MTrie {
	if q.count == 0 {
		return nil
	}
	tries := make([]*payloadless.MTrie, q.count)
	if q.tail >= q.count { // contiguous segment
		head := q.tail - q.count
		copy(tries, q.ts[head:q.tail])
	} else { // wrapped around
		head := q.capacity - q.count + q.tail
		n := copy(tries, q.ts[head:])
		copy(tries[n:], q.ts[:q.tail])
	}
	return tries
}

// Count returns the current element count.
func (q *PayloadlessTrieQueue) Count() int {
	return q.count
}

func (q *PayloadlessTrieQueue) isFull() bool {
	return q.count == q.capacity
}
