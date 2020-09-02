package queue

import (
	"time"
)

type item struct {
	message  interface{}
	priority int // The priority of the item in the queue.
	// The index is needed by update and is maintained by the heap.Interface methods.
	index     int   // The index of the item in the heap.
	timestamp int64 // timestamp to maintain insertions order for items with the same priority and for telemetry

}

// A priorityQueue implements heap.Interface and holds Items.
type priorityQueue []*item

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	if pq[i].priority > pq[j].priority {
		return true
	}
	if pq[i].priority < pq[j].priority {
		return false
	}
	// if both items have the same priority, then pop the oldest
	return pq[i].timestamp < pq[j].timestamp
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*item)
	item.index = n
	item.timestamp = time.Now().UnixNano()
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}
