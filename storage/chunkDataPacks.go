package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataPacks represents persistent storage for chunk data packs.
type ChunkDataPacks interface {

	// Store inserts the chunk header, keyed by chunk ID.
	Store(c *flow.ChunkDataPack) error

	// BatchStore inserts the chunk header, keyed by chunk ID into a given batch
	BatchStore(c *flow.ChunkDataPack, batch BatchStorage) error

	// Remove removes the chunk data for the given chunk ID, if it exists.
	Remove(chunkID flow.Identifier) error

	// ByChunkID returns the chunk data for the given a chunk ID.
	ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error)
}
