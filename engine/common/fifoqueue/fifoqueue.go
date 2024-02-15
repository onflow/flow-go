package fifoqueue

import (
	"fmt"
	mathbits "math/bits"
	"sync"

	"github.com/ef-ds/deque"
)

// CapacityUnlimited specifies the largest possible capacity for a FifoQueue.
// maximum value for platform-specific int: https://yourbasic.org/golang/max-min-int-uint/
const CapacityUnlimited = 1<<(mathbits.UintSize-1) - 1

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
// * the QueueLengthObserver must be non-blocking
type FifoQueue struct {
	mu             sync.RWMutex
	queue          deque.Deque
	maxCapacity    int
	lengthObserver QueueLengthObserver
}

// ConstructorOptions are optional arguments for the `NewFifoQueue`
// constructor to specify properties of the FifoQueue.
type ConstructorOption func(*FifoQueue) error

// QueueLengthObserver is a optional callback that can be provided to
// the `NewFifoQueue` constructor (via `WithLengthObserver` option).
type QueueLengthObserver func(int)

// WithLengthObserver is a constructor option for NewFifoQueue. Each time the
// queue's length changes, the queue calls the provided callback with the new
// length. By default, the QueueLengthObserver is a NoOp.
// CAUTION:
//   - QueueLengthObserver implementations must be non-blocking
//   - The values published to queue length observer might be in different order
//     than the actual length values at the time of insertion. This is a
//     performance optimization, which allows to reduce the duration during
//     which the queue is internally locked when inserting elements.
func WithLengthObserver(callback QueueLengthObserver) ConstructorOption {
	return func(queue *FifoQueue) error {
		if callback == nil {
			return fmt.Errorf("nil is not a valid QueueLengthObserver")
		}
		queue.lengthObserver = callback
		return nil
	}
}

// WithLengthMetricObserver attaches a length observer which calls the given observe function.
// It can be used to concisely bind a metrics observer implementing module.MempoolMetrics to the queue.
func WithLengthMetricObserver(resource string, observe func(resource string, length uint)) ConstructorOption {
	return WithLengthObserver(func(l int) {
		observe(resource, uint(l))
	})
}

// NewFifoQueue is the Constructor for FifoQueue
func NewFifoQueue(maxCapacity int, options ...ConstructorOption) (*FifoQueue, error) {
	if maxCapacity < 1 {
		return nil, fmt.Errorf("capacity for Fifo queue must be positive")
	}

	queue := &FifoQueue{
		maxCapacity:    maxCapacity,
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
		return q.queue.Len(), true
	}
	return length, false
}

// Front peeks message at the head of the queue (without removing the head).
func (q *FifoQueue) Head() (interface{}, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

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
	return event, q.queue.Len(), ok
}

// Len returns the current length of the queue.
func (q *FifoQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return q.queue.Len()
}
