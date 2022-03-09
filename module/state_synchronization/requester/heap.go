package requester

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization"
)

const CacheSize = 100

type blockEntry struct {
	index         int // used by heap for lookups
	height        uint64
	blockID       flow.Identifier
	executionData *state_synchronization.ExecutionData
}

func (e *blockEntry) purge() {
	e.executionData = nil
}

type NotificationHeap []*blockEntry

func (h NotificationHeap) Len() int {
	return len(h)
}

func (h NotificationHeap) Less(i, j int) bool {
	return h[i].height < h[j].height
}

func (h NotificationHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j

	if i > CacheSize {
		h[i].purge()
	}
	if j > CacheSize {
		h[j].purge()
	}
}

func (h *NotificationHeap) Push(x interface{}) {
	n := len(*h)
	entry := x.(*blockEntry)
	entry.index = n
	*h = append(*h, entry)

	if entry.index > CacheSize {
		entry.purge()
	}
}

func (h *NotificationHeap) Pop() interface{} {
	old := *h
	n := len(old)
	entry := old[n-1]
	old[n-1] = nil   // avoid memory leak
	entry.index = -1 // for safety
	*h = old[0 : n-1]
	return entry
}

func (h *NotificationHeap) PeekMin() *blockEntry {
	if len(*h) == 0 {
		return nil
	}

	arr := *h
	return arr[0]
}
