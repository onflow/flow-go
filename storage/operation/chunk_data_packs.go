package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertChunkDataPack inserts a chunk data pack keyed by chunk ID.
// any error are exceptions
func InsertChunkDataPack(w storage.Writer, c *storage.StoredChunkDataPack) error {
	return UpsertByKey(w, MakePrefix(codeChunkDataPack, c.ChunkID), c)
}

// RetrieveChunkDataPack retrieves a chunk data pack by chunk ID.
// it returns storage.ErrNotFound if the chunk data pack is not found
func RetrieveChunkDataPack(r storage.Reader, chunkID flow.Identifier, c *storage.StoredChunkDataPack) error {
	return RetrieveByKey(r, MakePrefix(codeChunkDataPack, chunkID), c)
}

// RemoveChunkDataPack removes the chunk data pack with the given chunk ID.
// any error are exceptions
func RemoveChunkDataPack(w storage.Writer, chunkID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeChunkDataPack, chunkID))
}
