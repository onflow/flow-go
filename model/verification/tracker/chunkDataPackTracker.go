package tracker

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// ChunkDataPackTracker represents a request tracker for a chunk data pack
type ChunkDataPackTracker struct {
	BlockID flow.Identifier
	ChunkID flow.Identifier
}

func (c *ChunkDataPackTracker) ID() flow.Identifier {
	return c.ChunkID
}

func (c *ChunkDataPackTracker) Checksum() flow.Identifier {
	return flow.MakeID(c)
}
