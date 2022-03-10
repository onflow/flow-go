package requester

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization"
)

type BlockEntry struct {
	BlockID       flow.Identifier
	Height        uint64
	ExecutionData *state_synchronization.ExecutionData

	index int // used by heap
}

// SetIndex is called by heap when moving the entry within the heap structure
// When the entry is moved farther than executionDataCacheSize from the top of the heap, its
// execution data is removed to enforce a cache size limit.
func (e *BlockEntry) SetIndex(index int) {
	// heap maintains index to be the sorted position within the data structure (0 is the smallest height)
	if index >= executionDataCacheSize {
		e.ExecutionData = nil
	}

	e.index = index
}
