package tracker

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// ExecutionStateTracker represents a request tracker for a chunk state
type ExecutionStateTracker struct {
	BlockID flow.Identifier
	ChunkID flow.Identifier
}

func (e *ExecutionStateTracker) ID() flow.Identifier {
	return e.ChunkID
}

func (e *ExecutionStateTracker) Checksum() flow.Identifier {
	return flow.MakeID(e)
}
