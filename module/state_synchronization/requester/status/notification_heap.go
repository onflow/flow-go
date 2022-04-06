package status

// NotificationHeap tracks outstanding notifications for the ExecutionDataRequester
// This uses the standard implementation for PriorityQueue from the go docs
// https://pkg.go.dev/container/heap
// with the addition of PeekMin()
type NotificationHeap []*BlockEntry

func (h NotificationHeap) Len() int {
	return len(h)
}

func (h NotificationHeap) Less(i, j int) bool {
	return h[i].Height < h[j].Height
}

func (h NotificationHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *NotificationHeap) Push(x interface{}) {
	*h = append(*h, x.(*BlockEntry))
}

func (h *NotificationHeap) Pop() interface{} {
	old := *h
	n := len(old)
	entry := old[n-1]
	old[n-1] = nil // avoid memory leak
	*h = old[0 : n-1]
	return entry
}

// PeekMin returns the entry with the smallest height without modifying the heap
func (h *NotificationHeap) PeekMin() *BlockEntry {
	if len(*h) == 0 {
		return nil
	}

	arr := *h
	return arr[0]
}
