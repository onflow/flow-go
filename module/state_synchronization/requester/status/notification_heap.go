package status

// notificationHeap tracks outstanding notifications for the ExecutionDataRequester
// This uses the standard implementation for PriorityQueue from the go docs
// https://pkg.go.dev/container/heap
// with the addition of PeekMin()
type notificationHeap []*BlockEntry

func (h notificationHeap) Len() int {
	return len(h)
}

func (h notificationHeap) Less(i, j int) bool {
	return h[i].Height < h[j].Height
}

func (h notificationHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *notificationHeap) Push(x interface{}) {
	*h = append(*h, x.(*BlockEntry))
}

func (h *notificationHeap) Pop() interface{} {
	old := *h
	n := len(old)
	entry := old[n-1]
	old[n-1] = nil // avoid memory leak
	*h = old[0 : n-1]
	return entry
}

// PeekMin returns the entry with the smallest height without modifying the heap
func (h *notificationHeap) PeekMin() *BlockEntry {
	if len(*h) == 0 {
		return nil
	}

	arr := *h
	return arr[0]
}
