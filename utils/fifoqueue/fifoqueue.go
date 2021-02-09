package fifoqueue

import (
	"fmt"
	mathbits "math/bits"

	"github.com/ef-ds/deque"
)

// FifoQueue implements a FIFO queue with max capacity and
// length observer.
type FifoQueue struct {
	queue          deque.Deque
	maxCapacity    int
	lengthObserver QueueLengthObserver
}

type Options func(*FifoQueue) error
type QueueLengthObserver func(int)

func WithCapacity(capacity int) Options {
	return func(queue *FifoQueue) error {
		if capacity < 1 {
			return fmt.Errorf("capacity for Fifo queue must be positive")
		}
		queue.maxCapacity = capacity
		return nil
	}
}

func WithLenMetric(observer QueueLengthObserver) Options {
	return func(queue *FifoQueue) error {
		if observer == nil {
			return fmt.Errorf("nil is not a valid QueueLengthObserver")
		}
		queue.lengthObserver = observer
		return nil
	}
}

func NewFifoQueue(options ...Options) (*FifoQueue, error) {
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
// If queue capacity is reached, the message is dropped.
func (q *FifoQueue) Push(value interface{}) {
	if q.queue.Len() < q.maxCapacity {
		q.queue.PushBack(value)
		q.lengthObserver(q.queue.Len())
	}
}

// Front peeks message at head of the queue.
func (q *FifoQueue) Front() (interface{}, bool) {
	return q.queue.Front()
}

// Pop removes the given message from the head of the queue
func (q *FifoQueue) Pop() (interface{}, bool) {
	event, ok := q.queue.PopFront()
	q.lengthObserver(q.queue.Len())
	if !ok {
		return nil, false
	}
	return event, true
}

func (q *FifoQueue) Len() int {
	return q.queue.Len()
}
