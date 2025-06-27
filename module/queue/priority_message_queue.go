package queue

import (
	"container/heap"
	"sync"
)

// PriorityMessageQueue is a thread-safe priority queue that provides a channel-based notification
// mechanism when items are inserted.
type PriorityMessageQueue[T any] struct {
	queue              PriorityQueue[T]
	smallerValuesFirst bool
	ch                 chan struct{}
	mu                 sync.RWMutex
}

// NewPriorityMessageQueue creates a new instance of PriorityMessageQueue.
func NewPriorityMessageQueue[T any](smallerValuesFirst bool) *PriorityMessageQueue[T] {
	return &PriorityMessageQueue[T]{
		queue:              PriorityQueue[T]{},
		smallerValuesFirst: smallerValuesFirst,
		ch:                 make(chan struct{}, 1),
	}
}

// Len returns the number of items currently in the queue.
func (mq *PriorityMessageQueue[T]) Len() int {
	mq.mu.RLock()
	defer mq.mu.RUnlock()

	return mq.queue.Len()
}

// Push adds a new item to the queue with the specified priority.
// A notification is sent on the channel if it's not already full.
func (mq *PriorityMessageQueue[T]) Push(item T, priority uint64) {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	// if smaller values are higher priority, invert the priority value since the heap will always
	// return the largest value first.
	if mq.smallerValuesFirst {
		priority = ^priority
	}

	heap.Push(&mq.queue, NewPriorityQueueItem(item, priority))

	select {
	case mq.ch <- struct{}{}:
	default:
	}
}

// Pop removes and immediately returns the highest priority item from the queue.
// If the queue is empty, false is returned.
// If multiple items have the same priority, the oldest one by insertion time is returned.
func (mq *PriorityMessageQueue[T]) Pop() (T, bool) {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	for mq.queue.Len() == 0 {
		var nilT T
		return nilT, false
	}

	item := heap.Pop(&mq.queue).(*PriorityQueueItem[T])
	return item.Message(), true
}

// Channel returns a signal channel that receives a signal when an item is inserted.
// This allows consumers to be notified of new items without polling.
func (mq *PriorityMessageQueue[T]) Channel() <-chan struct{} {
	return mq.ch
}
