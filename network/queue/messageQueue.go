package queue

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/onflow/flow-go/module"
)

// ErrQueueFull is returned when attempting to insert a message into a full queue.
var ErrQueueFull = errors.New("message queue is full")

type Priority int

const LowPriority = Priority(1)
const MediumPriority = Priority(5)
const HighPriority = Priority(10)

// DefaultMaxSize is the default maximum number of messages that can be queued.
// This limit prevents unbounded memory growth from message flooding attacks.
const DefaultMaxSize = 100_000

// MessagePriorityFunc - the callback function to derive priority of a message
type MessagePriorityFunc func(message any) (Priority, error)

// MessageQueue is the heap based priority queue implementation of the MessageQueue implementation.
// It enforces a maximum size limit to prevent unbounded memory growth from message flooding attacks.
type MessageQueue struct {
	pq           *priorityQueue
	cond         *sync.Cond
	priorityFunc MessagePriorityFunc
	ctx          context.Context
	metrics      module.NetworkInboundQueueMetrics
	maxSize      int
}

// Insert adds a message to the queue.
//
// Expected error returns during normal operation:
//   - [ErrQueueFull]: when the queue has reached its maximum capacity
func (mq *MessageQueue) Insert(message any) error {

	if err := mq.ctx.Err(); err != nil {
		return err
	}

	// determine the message priority
	priority, err := mq.priorityFunc(message)
	if err != nil {
		return fmt.Errorf("failed to derive message priority: %w", err)
	}

	// create the queue item
	item := &item{
		message:   message,
		priority:  int(priority),
		timestamp: time.Now(),
	}

	// lock the underlying mutex
	mq.cond.L.Lock()

	// check if queue is at capacity
	if mq.pq.Len() >= mq.maxSize {
		mq.cond.L.Unlock()
		return fmt.Errorf("queue at capacity (%d messages): %w", mq.maxSize, ErrQueueFull)
	}

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

func (mq *MessageQueue) Remove() any {
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

// NewMessageQueue creates a new bounded message queue with the specified maximum size.
// If maxSize is 0 or negative, DefaultMaxSize is used.
func NewMessageQueue(ctx context.Context, priorityFunc MessagePriorityFunc, metrics module.NetworkInboundQueueMetrics, maxSize int) *MessageQueue {
	if maxSize <= 0 {
		maxSize = DefaultMaxSize
	}
	var items = make([]*item, 0)
	pq := priorityQueue(items)
	mq := &MessageQueue{
		pq:           &pq,
		priorityFunc: priorityFunc,
		ctx:          ctx,
		metrics:      metrics,
		maxSize:      maxSize,
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
