package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// ChunkDataPacks represents persistent storage for chunk data packs.
type ChunkDataPacks interface {

	// Store inserts the chunk header, keyed by chunk ID.
	Store(c *flow.ChunkDataPack) error

	// Remove removes the chunk data for the given chunk ID, if it exists.
	Remove(chunkID flow.Identifier) error

	// ByID returns the chunk data for the given chunk ID.
	ByID(chunkID flow.Identifier) (*flow.ChunkDataPack, error)
}
