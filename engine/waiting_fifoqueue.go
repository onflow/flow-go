package engine

import (
	"context"
	"github.com/ef-ds/deque"
	mathbits "math/bits"
	"sync"
)

type WaitingFifoQueue struct {
	ctx         context.Context
	cond        *sync.Cond
	closed      bool
	queue       deque.Deque
	maxCapacity int
}

// NewWaitingFifoQueue is the Constructor for WaitingFifoQueue
func NewWaitingFifoQueue(ctx context.Context) (*WaitingFifoQueue, error) {
	// maximum value for platform-specific int: https://yourbasic.org/golang/max-min-int-uint/
	maxInt := 1<<(mathbits.UintSize-1) - 1

	mu := &sync.Mutex{}
	queue := &WaitingFifoQueue{
		maxCapacity: maxInt,
		cond:        sync.NewCond(mu),
	}

	go func() {
		<-ctx.Done()
		queue.cond.L.Lock()
		queue.closed = true
		queue.cond.Broadcast()
		queue.cond.L.Unlock()
	}()

	return queue, nil
}

// Push appends the given value to the tail of the queue.
// If queue capacity is reached, the message is silently dropped.
func (q *WaitingFifoQueue) Push(element interface{}) bool {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	length := q.queue.Len()
	if length < q.maxCapacity {
		q.queue.PushBack(element)
		q.cond.Signal()
		return true
	}
	return false
}

// Pop removes and returns the queue's head element.
// If the queue is empty, (nil, false) is returned.
func (q *WaitingFifoQueue) Pop() (interface{}, bool) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	for q.queue.Len() == 0 && !q.closed {
		q.cond.Wait()
	}
	if q.closed {
		return nil, false
	}

	event, _ := q.queue.PopFront()
	return event, true
}
