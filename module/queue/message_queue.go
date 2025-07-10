package queue

import (
	"sync"
)

// PriorityMessageQueue is a thread-safe priority queue that provides a channel-based notification
// mechanism when items are inserted. It wraps a PriorityQueue with synchronization
// and uses a channel to signal when new items are available.
// All methods are safe for concurrent access.
type PriorityMessageQueue[T any] struct {
	queue PriorityQueue[T]
	ch    chan struct{}
	mu    sync.RWMutex
}

// NewPriorityMessageQueue creates a new instance of PriorityMessageQueue.
func NewPriorityMessageQueue[T any]() *PriorityMessageQueue[T] {
	return &PriorityMessageQueue[T]{
		queue: NewPriorityQueue[T](),
		ch:    make(chan struct{}, 1),
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
// Items with lower priority take precedence over items with higher priority.
func (mq *PriorityMessageQueue[T]) Push(item T, priority uint64) {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	mq.queue.Push(NewPriorityQueueItem(item, priority, true))

	select {
	case mq.ch <- struct{}{}:
	default:
	}
}

// Pop removes and returns the highest priority item from the queue.
// The returned item will have the lowest priority value, or if multiple items have the same priority,
// the oldest one by insertion time.
//
// Returns:
//   - bool: false if the queue is empty
func (mq *PriorityMessageQueue[T]) Pop() (T, bool) {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	item, ok := mq.queue.Pop().(*PriorityQueueItem[T])
	if !ok {
		var nilT T
		return nilT, false
	}

	return item.Message(), true
}

// Channel returns a signal channel that receives a signal when an item is inserted.
// This allows consumers to be notified of new items without polling.
func (mq *PriorityMessageQueue[T]) Channel() <-chan struct{} {
	return mq.ch
}
