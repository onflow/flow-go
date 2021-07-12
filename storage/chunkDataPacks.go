package storage

import (
	"github.com/onflow/flow-go/model/flow"
	storagemodel "github.com/onflow/flow-go/storage/model"
)

// ChunkDataPacks represents persistent storage for chunk data packs.
type ChunkDataPacks interface {

	// Store inserts the chunk header, keyed by chunk ID.
	Store(c *storagemodel.StoredChunkDataPack) error

	// BatchStore inserts the chunk header, keyed by chunk ID into a given batch
	BatchStore(c *storagemodel.StoredChunkDataPack, batch BatchStorage) error

	// Remove removes the chunk data for the given chunk ID, if it exists.
	Remove(chunkID flow.Identifier) error

	// ByChunkID returns the chunk data for the given a chunk ID.
	ByChunkID(chunkID flow.Identifier) (*storagemodel.StoredChunkDataPack, error)
}
