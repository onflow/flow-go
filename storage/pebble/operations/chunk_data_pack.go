package operations

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertChunkDataPack inserts a chunk data pack keyed by chunk ID.
// any error are exceptions
func InsertChunkDataPack(sc *storage.StoredChunkDataPack) func(w pebble.Writer) error {
	key := makeKey(codeChunkDataPack, sc.ChunkID)
	return insert(key, sc)
}

// RetrieveChunkDataPack retrieves a chunk data pack by chunk ID.
// it returns storage.ErrNotFound if the chunk data pack is not found
func RetrieveChunkDataPack(chunkID flow.Identifier, sc *storage.StoredChunkDataPack) func(r pebble.Reader) error {
	key := makeKey(codeChunkDataPack, chunkID)
	return retrieve(key, sc)
}

// RemoveChunkDataPack removes the chunk data pack with the given chunk ID.
// any error are exceptions
func RemoveChunkDataPack(chunkID flow.Identifier) func(w pebble.Writer) error {
	key := makeKey(codeChunkDataPack, chunkID)
	return func(w pebble.Writer) error {
		return w.Delete(key, nil)
	}
}
