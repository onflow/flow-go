package queue

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

type headerInContext struct {
	Header *flow.Header
	ID     flow.Identifier
}

// FinalizedBlockQueue is a concurrency-safe queue of finalized block headers
// Every time a new header is added to the queue it verifies the complience of the header to the
// most recenlty added header.
// Under the hood a circular buffer with a limited capacity is used
type FinalizedBlockQueue struct {
	head, tail, capacity int
	isFull               bool
	headers              []headerInContext
	lock                 sync.RWMutex
	lastDequeuedHeader   *headerInContext
	lastQueuedHeader     *headerInContext
}

// NewFinalizedBlockQueue constructs a new FinalizedBlockQueue
// "capacity“ sets the limit on the maximum number of unconsumed block headers allowed in the queue
// "genesis“ is not inserted in the queue and is only used to to validate the very first incoming header
func NewFinalizedBlockQueue(
	capacity int,
	genesis *flow.Header,
) *FinalizedBlockQueue {
	return &FinalizedBlockQueue{
		capacity:           capacity,
		headers:            make([]headerInContext, capacity),
		lastDequeuedHeader: &headerInContext{genesis, genesis.ID()},
	}
}

// Enqueue checks the header compatibility and append a header to the queue
// A header is compatible if its parentID matches the ID of the last inserted header (or genesis)
// and its height is set right (last block's height plus one).
// If header not compatible or the queue has reached its capacity, corresponding errors are returned
func (ft *FinalizedBlockQueue) Enqueue(header *flow.Header) error {
	ft.lock.Lock()
	defer ft.lock.Unlock()

	if ft.isFull {
		return &QueueCapacityReachedError{ft.capacity}
	}

	parentID := ft.lastDequeuedHeader.ID
	lastHeight := ft.lastDequeuedHeader.Header.Height
	// check compliance
	if !ft.isEmpty() {
		parentID = ft.lastQueuedHeader.ID
		lastHeight = ft.lastQueuedHeader.Header.Height
	}

	if lastHeight+1 != header.Height || parentID != header.ParentID {
		return &NonCompliantHeaderError{
			lastHeight + 1,
			header.Height,
			parentID,
			header.ParentID,
		}
	}

	h := headerInContext{header, header.ID()}
	ft.headers[ft.tail] = h
	ft.lastQueuedHeader = &h
	ft.tail = (ft.tail + 1) % ft.capacity
	ft.isFull = ft.head == ft.tail
	return nil
}

// Peak returns the oldest header in the queue without removing it
// If the queue is empty it returns zero ID and nil header
func (ft *FinalizedBlockQueue) Peak() (flow.Identifier, *flow.Header) {
	ft.lock.RLock()
	defer ft.lock.RUnlock()

	if ft.isEmpty() {
		return flow.ZeroID, nil
	}
	header := ft.headers[ft.head]
	return header.ID, header.Header
}

// HasHeaders returns true if the queue is not empty and
// has more headers to be consumed
func (ft *FinalizedBlockQueue) HasHeaders() bool {
	ft.lock.RLock()
	defer ft.lock.RUnlock()

	return !ft.isEmpty()
}

// Dequeue removes the oldest header from the queue (without returning it)
// Dequeue from empty queue is a no-op
func (ft *FinalizedBlockQueue) Dequeue() {
	ft.lock.Lock()
	defer ft.lock.Unlock()

	if !ft.isEmpty() {
		ft.lastDequeuedHeader = &ft.headers[ft.head]
		ft.isFull = false
		ft.head = (ft.head + 1) % ft.capacity
	}
}

func (ft *FinalizedBlockQueue) isEmpty() bool {
	return ft.head == ft.tail && !ft.isFull
}
