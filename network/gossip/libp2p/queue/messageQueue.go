package queue

import (
	"container/heap"
	"sync"
)

// MessageQueue is the interface of the inbound message queue
type MessageQueue interface {
	// Insert inserts the message in queue
	Insert(message interface{}) error
	// Remove removes the message from the queue in priority order. If no message is found, this call blocks.
	// If two messages have the same priority, items are de-queued in insertion order
	Remove() interface{}
	// Len gives the current length of the queue
	Len() int
}

type Priority int

const Priority_1 Priority = 1
const Priority_2 Priority = 2
const Priority_3 Priority = 3
const Priority_4 Priority = 4
const Priority_5 Priority = 5
const Priority_6 Priority = 6
const Priority_7 Priority = 7
const Priority_8 Priority = 8
const Priority_9 Priority = 9
const Priority_10 Priority = 10

const Low_Priority = Priority_1
const Medium_Priority = Priority_5
const High_Priority = Priority_10

// MessagePriorityFunc - the callback function to derive priority of a message
type MessagePriorityFunc func(message interface{}) Priority

// MessageQueueImpl is the heap based priority queue implementation of the MessageQueue implementation
type MessageQueueImpl struct {
	pq           *priorityQueue
	cond         *sync.Cond
	priorityFunc MessagePriorityFunc
}

func (mq *MessageQueueImpl) Insert(message interface{}) error {

	// determine the message priority
	priority := mq.priorityFunc(message)

	// create the queue item
	item := &item{
		message:  message,
		priority: int(priority),
	}

	// lock the underlying mutex
	mq.cond.L.Lock()

	// push message to the underlying priority queue
	heap.Push(mq.pq, item)

	// signal a waiting routine that a message is now available
	mq.cond.Signal()

	// unlock the underlying mutex
	mq.cond.L.Unlock()

	return nil
}

func (mq *MessageQueueImpl) Remove() interface{} {
	mq.cond.L.Lock()
	defer mq.cond.L.Unlock()
	for mq.pq.Len() == 0 {
		mq.cond.Wait()
	}
	return heap.Pop(mq.pq).(*item).message
}

func (mq *MessageQueueImpl) Len() int {
	mq.cond.L.Lock()
	defer mq.cond.L.Unlock()
	return mq.pq.Len()
}

func NewMessageQueue(priorityFunc MessagePriorityFunc) *MessageQueueImpl {
	var items = make([]*item, 0)
	pq := priorityQueue(items)
	mq := &MessageQueueImpl{
		pq:           &pq,
		priorityFunc: priorityFunc,
	}
	m := sync.Mutex{}
	mq.cond = sync.NewCond(&m)
	return mq
}
