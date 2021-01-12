package matching

import "github.com/onflow/flow-go/model/flow"

// RequestTracker tracks the number of approval-requests sent per Chunk.
// This object is not concurrency safe.
type RequestTracker struct {
	index map[flow.Identifier]map[uint64]uint
}

// NewRequestTracker instantiates a new RequestTracker.
func NewRequestTracker() *RequestTracker {
	return &RequestTracker{
		index: make(map[flow.Identifier]map[uint64]uint),
	}
}

// GetAll returns a map with the number of approval-requests sent per chunk,
// indexed by result ID and chunk index.
func (rt *RequestTracker) GetAll() map[flow.Identifier]map[uint64]uint {
	return rt.index
}

// Get returns the number of request-approvals sent for a specific chunk.
func (rt *RequestTracker) Get(resultID flow.Identifier, chunkIndex uint64) uint {
	return rt.index[resultID][chunkIndex]
}

// Increment increments the number of request-approvals sent for a specific
// chunk.
func (rt *RequestTracker) Increment(resultID flow.Identifier, chunkIndex uint64) {
	_, ok := rt.index[resultID]
	if !ok {
		rt.index[resultID] = make(map[uint64]uint)
	}
	rt.index[resultID][chunkIndex] = rt.index[resultID][chunkIndex] + 1
}

// Remove removes all entries pertaining to an ExecutionResult from the
// RequestTracker.
func (rt *RequestTracker) Remove(resultID flow.Identifier) {
	delete(rt.index, resultID)
}
