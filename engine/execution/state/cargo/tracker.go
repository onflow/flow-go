package cargo

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

// TODO add error type duplication found
// mismatched

type headerWithID struct {
	Header *flow.Header
	ID     flow.Identifier
}

// FinTracker holds on to an ordered list of consumable headers
// it checks that blocks are forming a chain
type FinalizedBlockTracker struct {
	start, end, capacity int
	headers              []headerWithID
	lock                 sync.RWMutex
	lastDequeuedHeader   *headerWithID
	lastQueuedHeader     *headerWithID
}

// startHeader won't be included, it would just be used for validation initially
// when queue is empty
func NewFinalizedBlockTracker(
	capacity int,
	genesis *flow.Header,
) *FinalizedBlockTracker {
	return &FinalizedBlockTracker{
		capacity:           capacity,
		headers:            make([]headerWithID, capacity),
		lastDequeuedHeader: &headerWithID{genesis, genesis.ID()},
	}
}

func (ft *FinalizedBlockTracker) size() int {
	return ft.end - ft.start
}

func (ft *FinalizedBlockTracker) isEmpty() bool {
	return ft.start == ft.end
}

func (ft *FinalizedBlockTracker) isFull() bool {
	return ft.size() == ft.capacity
}

// Enqueue append a header to the queue (returns error if reached capacity)
func (ft *FinalizedBlockTracker) Enqueue(header *flow.Header) error {
	ft.lock.Lock()
	defer ft.lock.Unlock()

	if ft.isFull() {
		return fmt.Errorf("fin tracker queue is full (capacity: %d)", ft.capacity)
	}

	parentID := ft.lastDequeuedHeader.ID
	// check compliance
	if !ft.isEmpty() {
		parentID = ft.lastQueuedHeader.ID
	}

	if parentID != header.ParentID {
		return fmt.Errorf("block finalization events are not order preserving, last block ID: %x, new block's parent ID: %x", parentID, header.ParentID)
	}
	h := headerWithID{header, header.ID()}
	ft.headers[ft.end] = h
	ft.lastQueuedHeader = &h
	ft.end = (ft.end + 1) % ft.capacity
	return nil
}

// Peak returns the oldest header in the queue without removing it
// if queue is empty returns zero ID and nil header
func (ft *FinalizedBlockTracker) Peak() (flow.Identifier, *flow.Header) {
	ft.lock.RLock()
	defer ft.lock.RUnlock()

	if ft.isEmpty() {
		return flow.ZeroID, nil
	}
	header := ft.headers[ft.start]
	return header.ID, header.Header
}

// Dequeue removes the oldest header from the queue (without returning it)
// Dequeue from empty queue is a no-op
func (ft *FinalizedBlockTracker) Dequeue() {
	ft.lock.Lock()
	defer ft.lock.Unlock()

	if !ft.isEmpty() {
		ft.lastDequeuedHeader = &ft.headers[ft.start]
		ft.start = (ft.start + 1) % ft.capacity
	}
}
