package matching

import (
	"fmt"
	math_bits "math/bits"
	"sync"

	"github.com/ef-ds/deque"

	"github.com/onflow/flow-go/model/flow"
)

// FifoQueue implements a concurrency-safe fifo queue.
// For practical purposes, the queue does not block.
type FifoQueue struct {
	sync.RWMutex
	queue          deque.Deque
	maxCapacity    int
	lengthObserver QueueLengthObserver
}

type FifoQueueOptions func(*FifoQueue) error
type QueueLengthObserver func(int)

type element struct {
	OriginID flow.Identifier
	Msg      interface{}
}

func WithCapacity(capacity int) FifoQueueOptions {
	return func(queue *FifoQueue) error {
		if capacity < 1 {
			return fmt.Errorf("capacity for Fifo queue must be positive")
		}
		queue.maxCapacity = capacity
		return nil
	}
}

func WithLenMetric(observer QueueLengthObserver) FifoQueueOptions {
	return func(queue *FifoQueue) error {
		if observer == nil {
			return fmt.Errorf("nil is not a valid QueueLengthObserver")
		}
		queue.lengthObserver = observer
		return nil
	}
}

func NewFifoQueue(options ...FifoQueueOptions) (*FifoQueue, error) {
	// maximum value for platform-specific int: https://yourbasic.org/golang/max-min-int-uint/
	maxInt := 1<<(math_bits.UintSize-1) - 1

	queue := &FifoQueue{
		maxCapacity:    maxInt,
		lengthObserver: func(int) { /* noop */ },
	}
	for _, opt := range options {
		err := opt(queue)
		if err != nil {
			return nil, fmt.Errorf("failed to apply constructor option to fifo queue: %w", err)
		}
	}
	return queue, nil
}

// Push appends the given message to the tail of the queue.
// If queue capacity is reached, the message is dropped.
func (q *FifoQueue) Push(originID flow.Identifier, message interface{}) {
	event := &element{
		OriginID: originID,
		Msg:      message,
	}
	q.Lock()
	if q.queue.Len() < q.maxCapacity {
		q.queue.PushBack(event)
		q.lengthObserver(q.queue.Len())
	}
	q.Unlock()
}

// Pop removes the given message to the tail of the queue
func (q *FifoQueue) Pop() (flow.Identifier, interface{}, bool) {
	q.Lock()
	event, ok := q.queue.PopFront()
	q.lengthObserver(q.queue.Len())
	q.Unlock()

	if !ok {
		return flow.ZeroID, nil, false
	}
	elmnt := event.(*element)
	return elmnt.OriginID, elmnt.Msg, true
}

func (q *FifoQueue) Len() int {
	q.RLock()
	defer q.RUnlock()
	return q.queue.Len()
}
