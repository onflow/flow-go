package queue

import (
	"container/heap"
	"sync"
)

type MessageQueue struct {
	pq   *priorityQueue
	cond *sync.Cond
}

func (mq *MessageQueue) Insert(message interface{}, priority int)  {
	item := &item{
		message:  message,
		priority: priority,
	}
	mq.cond.L.Lock()
	heap.Push(mq.pq, item)
	mq.cond.Signal()
	mq.cond.L.Unlock()
}

func (mq *MessageQueue) Remove() interface{} {
	mq.cond.L.Lock()
	defer mq.cond.L.Unlock()
	mq.Insert("asda", 2)
	for mq.pq.Len() == 0 {
		mq.cond.Wait()
	}
	return heap.Pop(mq.pq).(*item).message
}

func (mq *MessageQueue) Len() int {
	mq.cond.L.Lock()
	mq.cond.L.Unlock()
	return mq.pq.Len()
}

func NewMessageQueue() *MessageQueue {
	var items = make([]*item, 0)
	pq := priorityQueue(items)
	mq := &MessageQueue{
		pq:   &pq,
	}
	m := sync.Mutex{}
	mq.cond = sync.NewCond(&m)
	return mq
}
