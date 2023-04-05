package cargo

import (
	"errors"
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

var ErrHeaderOrder = errors.New("headers are received in a non-compliant order")
var ErrCapacityReached = errors.New("queue capacity has reached")

// TODO deal with duplications

type headerWithID struct {
	Header *flow.Header
	ID     flow.Identifier
}

// FinalizedBlockQueue maintains an ordered queue of consumable block headers
// it also verifies that headers are added to the queue in the right order
// under the hood it uses a circular buffer with a limited size to keep these block headers
// if it reaches the limit it prevents adding more block headers
type FinalizedBlockQueue struct {
	start, end, capacity int
	headers              []headerWithID
	lock                 sync.RWMutex
	lastDequeuedHeader   *headerWithID
	lastQueuedHeader     *headerWithID
}

// NewFinalizedBlockQueue constructs a new FinalizedBlockQueue
// capacity limits the number of unconsumed headers in the queue
// genesis is used to validate the very first header that is going to be added to the queue
func NewFinalizedBlockQueue(
	capacity int,
	genesis *flow.Header,
) *FinalizedBlockQueue {
	return &FinalizedBlockQueue{
		capacity:           capacity,
		headers:            make([]headerWithID, capacity),
		lastDequeuedHeader: &headerWithID{genesis, genesis.ID()},
	}
}

// Enqueue append a header to the queue given that header is compatible
// with previously added header (parentID matches and height is right)
// it returns an error if the queue has reached the capacity
func (ft *FinalizedBlockQueue) Enqueue(header *flow.Header) error {
	ft.lock.Lock()
	defer ft.lock.Unlock()

	if ft.isFull() {
		return ErrCapacityReached
	}

	parentID := ft.lastDequeuedHeader.ID
	lastHeight := ft.lastDequeuedHeader.Header.Height
	// check compliance
	if !ft.isEmpty() {
		parentID = ft.lastQueuedHeader.ID
		lastHeight = ft.lastQueuedHeader.Header.Height
	}

	if parentID != header.ParentID || lastHeight+1 != header.Height {
		return ErrHeaderOrder
	}

	h := headerWithID{header, header.ID()}
	ft.headers[ft.end] = h
	ft.lastQueuedHeader = &h
	ft.end = (ft.end + 1) % ft.capacity
	return nil
}

// Peak returns the oldest header in the queue without removing it
// if queue is empty returns zero ID and nil header
func (ft *FinalizedBlockQueue) Peak() (flow.Identifier, *flow.Header) {
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
func (ft *FinalizedBlockQueue) Dequeue() {
	ft.lock.Lock()
	defer ft.lock.Unlock()

	if !ft.isEmpty() {
		ft.lastDequeuedHeader = &ft.headers[ft.start]
		ft.start = (ft.start + 1) % ft.capacity
	}
}

func (ft *FinalizedBlockQueue) size() int {
	return ft.end - ft.start
}

func (ft *FinalizedBlockQueue) isEmpty() bool {
	return ft.start == ft.end
}

func (ft *FinalizedBlockQueue) isFull() bool {
	return ft.size() == ft.capacity
}
