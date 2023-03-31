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

	// ByChunkID returns the chunk data for the given a chunk ID.
	ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error)

	// BatchRemove removes ChunkDataPack c keyed by its ChunkID in provided batch
	// No errors are expected during normal operation, even if no entries are matched.
	// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchRemove(chunkID flow.Identifier, batch BatchStorage) error
}
