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
type FinalizedBlockQueue struct {
	headers            []headerInContext
	lock               sync.RWMutex
	lastDequeuedHeader *headerInContext
	lastQueuedHeader   *headerInContext
}

// NewFinalizedBlockQueue constructs a new FinalizedBlockQueue
// "genesis“ is not inserted in the queue and is only used to to validate the very first incoming header
func NewFinalizedBlockQueue(
	genesis *flow.Header,
) *FinalizedBlockQueue {
	return &FinalizedBlockQueue{
		headers:            make([]headerInContext, 0),
		lastDequeuedHeader: &headerInContext{genesis, genesis.ID()},
	}
}

// Enqueue checks the header compatibility and append a header to the queue
// A header is compatible if its parentID matches the ID of the last inserted header (or genesis)
// and its height is set right (last block's height plus one).
// an error is returned if header not compatible
func (ft *FinalizedBlockQueue) Enqueue(header *flow.Header) error {
	ft.lock.Lock()
	defer ft.lock.Unlock()

	expectedHeight := ft.lastDequeuedHeader.Header.Height + 1
	expectedParentID := ft.lastDequeuedHeader.ID
	if len(ft.headers) > 0 {
		expectedHeight = ft.lastQueuedHeader.Header.Height + 1
		expectedParentID = ft.lastQueuedHeader.ID
	}
	if expectedHeight != header.Height || expectedParentID != header.ParentID {
		return &NonCompliantHeaderError{
			expectedHeight,
			header.Height,
			expectedParentID,
			header.ParentID,
		}
	}

	h := headerInContext{header, header.ID()}
	ft.headers = append(ft.headers, h)
	ft.lastQueuedHeader = &h
	return nil
}

// Peak returns the oldest header in the queue without removing it
// If the queue is empty it returns zero ID and nil header
func (ft *FinalizedBlockQueue) Peak() (flow.Identifier, *flow.Header) {
	ft.lock.RLock()
	defer ft.lock.RUnlock()

	if len(ft.headers) == 0 {
		return flow.ZeroID, nil
	}

	header := ft.headers[0]
	return header.ID, header.Header
}

// HasHeaders returns true if the queue is not empty and
// has more headers to be consumed
func (ft *FinalizedBlockQueue) HasHeaders() bool {
	ft.lock.RLock()
	defer ft.lock.RUnlock()

	return len(ft.headers) > 0
}

// Dequeue removes the oldest header from the queue (without returning it)
// Dequeue from empty queue is a no-op
func (ft *FinalizedBlockQueue) Dequeue() {
	ft.lock.Lock()
	defer ft.lock.Unlock()

	if len(ft.headers) > 0 {
		ft.lastDequeuedHeader = &ft.headers[0]
		ft.headers = ft.headers[1:]
	}
}
