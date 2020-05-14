package tracker

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// ChunkDataPackTracker represents a request tracker for a chunk data pack
type ChunkDataPackTracker struct {
	BlockID flow.Identifier
	ChunkID flow.Identifier
	Counter uint // keeps track of number of retries
}

func (c *ChunkDataPackTracker) ID() flow.Identifier {
	return c.ChunkID
}

func (c *ChunkDataPackTracker) Checksum() flow.Identifier {
	return flow.MakeID(c)
}

// NewChunkDataPackTracker creates a new ChunkDataPack tracker structure out of the chunkID and
// blockID. It also sets the counter value of tracker to one.
func NewChunkDataPackTracker(chunkID, blockID flow.Identifier) *ChunkDataPackTracker {
	return &ChunkDataPackTracker{
		ChunkID: chunkID,
		BlockID: blockID,
		Counter: 1,
	}
}
