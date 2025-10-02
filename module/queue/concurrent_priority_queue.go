package queue

import (
	"container/heap"
	"sync"

	"github.com/onflow/flow-go/engine"
)

// ConcurrentPriorityQueue is a thread-safe priority queue that provides a channel-based notification
// mechanism when items are inserted.
// All methods are safe for concurrent access.
type ConcurrentPriorityQueue[T any] struct {
	queue              PriorityQueue[T]
	smallerValuesFirst bool
	trapdoor           *engine.Trapdoor
	mu                 sync.RWMutex
}

// NewConcurrentPriorityQueue creates a new instance of ConcurrentPriorityQueue.
// If smallerValuesFirst is true, inverts the priority so items with lower values take precedence.
func NewConcurrentPriorityQueue[T any](smallerValuesFirst bool) *ConcurrentPriorityQueue[T] {
	return &ConcurrentPriorityQueue[T]{
		queue:              PriorityQueue[T]{},
		smallerValuesFirst: smallerValuesFirst,
		trapdoor:           engine.NewTrapdoor(),
	}
}

// Len returns the number of items currently in the queue.
func (mq *ConcurrentPriorityQueue[T]) Len() int {
	mq.mu.RLock()
	defer mq.mu.RUnlock()

	return mq.queue.Len()
}

// Push adds a new item to the queue with the specified priority.
// A notification is sent on the channel if it's not already full.
func (mq *ConcurrentPriorityQueue[T]) Push(item T, priority uint64) {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	// if smaller values are higher priority, invert the priority value since the heap will always
	// return the largest value first.
	if mq.smallerValuesFirst {
		priority = ^priority
	}

	heap.Push(&mq.queue, NewPriorityQueueItem(item, priority))

	mq.trapdoor.Activate()
}

// Pop removes and immediately returns the highest priority item from the queue.
// If the queue is empty, false is returned.
// If multiple items have the same priority, the oldest one by insertion time is returned.
func (mq *ConcurrentPriorityQueue[T]) Pop() (T, bool) {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	if mq.queue.Len() == 0 {
		var nilT T
		return nilT, false
	}

	item := heap.Pop(&mq.queue).(*PriorityQueueItem[T])
	return item.Message(), true
}

// Channel returns a signal channel that receives a signal when an item is inserted.
// This allows consumers to be notified of new items without polling.
func (mq *ConcurrentPriorityQueue[T]) Channel() <-chan struct{} {
	return mq.trapdoor.Channel()
}
