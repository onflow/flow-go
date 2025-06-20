package queue

import (
	"container/heap"
	"time"
)

// PriorityQueueItem is a generic item in the priority queue.
// Each item contains a message, priority value, and metadata for queue management.
// PriorityQueueItems are immutable once created and safe for concurrent access.
type PriorityQueueItem[T any] struct {
	// message is the actual item in the queue.
	message T

	// priority is the priority of the item in the queue.
	// Higher priority values are dequeued first.
	priority uint64

	// index is the index of the item in the heap.
	// The index is required by update() and is maintained by the heap.Interface methods.
	index int

	// timestamp to maintain insertions order for items with the same priority and for telemetry
	timestamp time.Time
}

// NewPriorityQueueItem creates a new PriorityQueueItem with the given message and priority.
//
// Parameters:
//   - message: the actual item to store in the queue
//   - priority: the priority value for the item (higher values dequeued first)
//   - invertPriorityOrder: if true, inverts the priority so lower values are considered higher priority
//
// Returns:
//   - *PriorityQueueItem[T]: the newly created item
//
// Concurrency safety:
//   - Safe for concurrent access
func NewPriorityQueueItem[T any](message T, priority uint64, invertPriorityOrder bool) *PriorityQueueItem[T] {
	if invertPriorityOrder {
		priority = ^priority
	}

	return &PriorityQueueItem[T]{
		message:   message,
		priority:  priority,
		index:     -1, // index is set when the item is pushed to the heap
		timestamp: time.Now(),
	}
}

// Message returns the message stored in the item.
//
// Returns:
//   - T: the message stored in the item
//
// Concurrency safety:
//   - Safe for concurrent access
func (item *PriorityQueueItem[T]) Message() T {
	return item.message
}

var _ heap.Interface = (*PriorityQueue[any])(nil)

// PriorityQueue implements heap.Interface and holds PriorityQueueItems.
// It provides a priority queue where items with higher priority values
// are dequeued first. For items with equal priority, the oldest item (by insertion time)
// is dequeued first.
//
// All exported methods are NOT safe for concurrent access. Caller must implement their own synchronization.
type PriorityQueue[T any] []*PriorityQueueItem[T]

func NewPriorityQueue[T any]() PriorityQueue[T] {
	return PriorityQueue[T]{}
}

// Len returns the number of items in the priority queue.
//
// Returns:
//   - int: the number of items in the queue
func (pq PriorityQueue[T]) Len() int { return len(pq) }

// Less determines the ordering of items in the priority queue.
// PriorityQueueItems with higher priority values come first. For items with equal priority,
// the oldest item (by insertion timestamp) comes first.
//
// Parameters:
//   - i: index of the first item to compare
//   - j: index of the second item to compare
//
// Returns:
//   - bool: true if item at index i should come before item at index j
func (pq PriorityQueue[T]) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	if pq[i].priority > pq[j].priority {
		return true
	}
	if pq[i].priority < pq[j].priority {
		return false
	}
	// if both items have the same priority, then pop the oldest
	return pq[i].timestamp.Before(pq[j].timestamp)
}

// Swap exchanges the items at the given indices and updates their heap indices.
//
// Parameters:
//   - i: index of the first item to swap
//   - j: index of the second item to swap
func (pq PriorityQueue[T]) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// Push adds an item to the priority queue.
// The item's index is automatically set to its position in the heap.
//
// Parameters:
//   - x: the item to add to the queue (must be *PriorityQueueItem[T])
func (pq *PriorityQueue[T]) Push(x any) {
	n := len(*pq)
	item, ok := x.(*PriorityQueueItem[T])
	if !ok {
		return
	}
	item.index = n
	*pq = append(*pq, item)
}

// Pop removes and returns the highest priority item from the queue.
// The returned item will have the highest priority value, or if multiple items
// have the same priority, the oldest one by insertion time.
//
// Returns:
//   - any: the highest priority item, or nil if the queue is empty
func (pq *PriorityQueue[T]) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}
