package matching

import "github.com/onflow/flow-go/model/flow"

type RequestTracker struct {
	index map[flow.Identifier]map[uint64]uint
}

func NewRequestTracker() *RequestTracker {
	return &RequestTracker{
		index: make(map[flow.Identifier]map[uint64]uint),
	}
}

func (rt *RequestTracker) GetAll() map[flow.Identifier]map[uint64]uint {
	return rt.index
}

func (rt *RequestTracker) Get(resultID flow.Identifier, chunkIndex uint64) uint {
	return rt.index[resultID][chunkIndex]
}

func (rt *RequestTracker) Increment(resultID flow.Identifier, chunkIndex uint64) {
	_, ok := rt.index[resultID]
	if !ok {
		rt.index[resultID] = make(map[uint64]uint)
	}
	rt.index[resultID][chunkIndex] = rt.index[resultID][chunkIndex] + 1
}

func (rt *RequestTracker) Remove(resultID flow.Identifier) {
	delete(rt.index, resultID)
}
