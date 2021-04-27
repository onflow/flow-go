package fifoqueue

import (
	"fmt"
	mathbits "math/bits"
	"sync"

	"github.com/ef-ds/deque"
)

// FifoQueue implements a FIFO queue with max capacity and length observer.
// Elements that exceeds the queue's max capacity are silently dropped.
// By default, the theoretical capacity equals to the largest `int` value
// (platform dependent). Capacity can be set at construction time via the
// option `WithCapacity`.
// Each time the queue's length changes, the QueueLengthObserver is called
// with the new length. By default, the QueueLengthObserver is a NoOp.
// A single QueueLengthObserver can be set at construction time via the
// option `WithLengthObserver`.
//
// Caution:
// * The queue is NOT concurrency safe.
// * the QueueLengthObserver must be non-blocking
type FifoQueue struct {
	mu             sync.RWMutex
	queue          deque.Deque
	maxCapacity    int
	lengthObserver QueueLengthObserver
}

// ConstructorOptions can are optional arguments for the `NewFifoQueue`
// constructor to specify properties of the FifoQueue.
type ConstructorOption func(*FifoQueue) error

// QueueLengthObserver is a callback that can optionally provided
// to the `NewFifoQueue` constructor (via `WithLengthObserver` option).
type QueueLengthObserver func(int)

// WithCapacity is a constructor option for NewFifoQueue. It specifies the
// max number of elements the queue can hold. By default, the theoretical
// capacity equals to the largest `int` value (platform dependent).
// The WithCapacity option overrides the previous value (default value or
// value specified by previous option).
func WithCapacity(capacity int) ConstructorOption {
	return func(queue *FifoQueue) error {
		if capacity < 1 {
			return fmt.Errorf("capacity for Fifo queue must be positive")
		}
		queue.maxCapacity = capacity
		return nil
	}
}

// WithLengthObserver is a constructor option for NewFifoQueue. Each time the
// queue's length changes, the queue calls the provided callback with the new
// length. By default, the QueueLengthObserver is a NoOp.
// Caution: the QueueLengthObserver callback must be non-blocking
func WithLengthObserver(callback QueueLengthObserver) ConstructorOption {
	return func(queue *FifoQueue) error {
		if callback == nil {
			return fmt.Errorf("nil is not a valid QueueLengthObserver")
		}
		queue.lengthObserver = callback
		return nil
	}
}

// Constructor for FifoQueue
func NewFifoQueue(options ...ConstructorOption) (*FifoQueue, error) {
	// maximum value for platform-specific int: https://yourbasic.org/golang/max-min-int-uint/
	maxInt := 1<<(mathbits.UintSize-1) - 1

	queue := &FifoQueue{
		maxCapacity:    maxInt,
		lengthObserver: func(int) { /* noop */ },
	}
	for _, opt := range options {
		err := opt(queue)
		if err != nil {
			return nil, fmt.Errorf("failed to apply constructor option to fifoqueue queue: %w", err)
		}
	}
	return queue, nil
}

// Push appends the given value to the tail of the queue.
// If queue capacity is reached, the message is silently dropped.
func (q *FifoQueue) Push(element interface{}) bool {
	length, pushed := q.push(element)

	if pushed {
		q.lengthObserver(length)
	}
	return pushed
}

func (q *FifoQueue) push(element interface{}) (int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	length := q.queue.Len()
	if length < q.maxCapacity {
		q.queue.PushBack(element)
		return length, true
	}
	return length, false
}

// Front peeks message at the head of the queue (without removing the head).
func (q *FifoQueue) Front() (interface{}, bool) {
	q.mu.RLock()
	defer q.mu.RLock()

	return q.queue.Front()
}

// Pop removes and returns the queue's head element.
// If the queue is empty, (nil, false) is returned.
func (q *FifoQueue) Pop() (interface{}, bool) {
	event, length, ok := q.pop()
	if !ok {
		return nil, false
	}

	q.lengthObserver(length)
	return event, true
}

func (q *FifoQueue) pop() (interface{}, int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	event, ok := q.queue.PopFront()
	length := q.queue.Len()
	return event, length, ok
}

// Len returns the current length of the queue.
func (q *FifoQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return q.queue.Len()
}
