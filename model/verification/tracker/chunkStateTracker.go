package tracker

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// ChunkStateTracker represents a request tracker for a chunk state
type ChunkStateTracker struct {
	BlockID flow.Identifier
	ChunkID flow.Identifier
}

func (e *ChunkStateTracker) ID() flow.Identifier {
	return e.ChunkID
}

func (e *ChunkStateTracker) Checksum() flow.Identifier {
	return flow.MakeID(e)
}
