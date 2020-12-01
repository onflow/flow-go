package queue

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/onflow/flow-go/module"
)

type Priority int

const LowPriority = Priority(1)
const MediumPriority = Priority(5)
const HighPriority = Priority(10)

// MessagePriorityFunc - the callback function to derive priority of a message
type MessagePriorityFunc func(message interface{}) (Priority, error)

// MessageQueue is the heap based priority queue implementation of the MessageQueue implementation
type MessageQueue struct {
	pq           *priorityQueue
	cond         *sync.Cond
	priorityFunc MessagePriorityFunc
	ctx          context.Context
	metrics      module.NetworkMetrics
}

func (mq *MessageQueue) Insert(message interface{}) error {

	if err := mq.ctx.Err(); err != nil {
		return err
	}

	// determine the message priority
	priority, err := mq.priorityFunc(message)
	if err != nil {
		return fmt.Errorf("failed to dervie message priority: %w", err)
	}

	// create the queue item
	item := &item{
		message:   message,
		priority:  int(priority),
		timestamp: time.Now(),
	}

	// lock the underlying mutex
	mq.cond.L.Lock()

	// push message to the underlying priority queue
	heap.Push(mq.pq, item)

	// record metrics
	mq.metrics.MessageAdded(item.priority)

	// signal a waiting routine that a message is now available
	mq.cond.Signal()

	// unlock the underlying mutex
	mq.cond.L.Unlock()

	return nil
}

func (mq *MessageQueue) Remove() interface{} {
	mq.cond.L.Lock()
	defer mq.cond.L.Unlock()
	for mq.pq.Len() == 0 {

		// if the context has been canceled, don't wait
		if err := mq.ctx.Err(); err != nil {
			return nil
		}

		mq.cond.Wait()
	}
	item := heap.Pop(mq.pq).(*item)

	// record metrics
	mq.metrics.QueueDuration(time.Since(item.timestamp), item.priority)
	mq.metrics.MessageRemoved(item.priority)

	return item.message
}

func (mq *MessageQueue) Len() int {
	mq.cond.L.Lock()
	defer mq.cond.L.Unlock()
	return mq.pq.Len()
}

func NewMessageQueue(ctx context.Context, priorityFunc MessagePriorityFunc, nm module.NetworkMetrics) *MessageQueue {
	var items = make([]*item, 0)
	pq := priorityQueue(items)
	mq := &MessageQueue{
		pq:           &pq,
		priorityFunc: priorityFunc,
		ctx:          ctx,
		metrics:      nm,
	}
	m := sync.Mutex{}
	mq.cond = sync.NewCond(&m)

	// kick off a go routine to unblock queue readers on shutdown
	go func() {
		<-ctx.Done()
		// unblock receive
		mq.cond.Broadcast()
	}()

	return mq
}
