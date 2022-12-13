package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	badgermodel "github.com/onflow/flow-go/storage/badger/model"
)

// InsertChunkDataPack inserts a chunk data pack keyed by chunk ID.
func InsertChunkDataPack(c *badgermodel.StoredChunkDataPack) func(*badger.Txn) error {
	return insert(makePrefix(codeChunkDataPack, c.ChunkID), c)
}

// BatchInsertChunkDataPack inserts a chunk data pack keyed by chunk ID into a batch
func BatchInsertChunkDataPack(c *badgermodel.StoredChunkDataPack) func(batch *badger.WriteBatch) error {
	return batchWrite(makePrefix(codeChunkDataPack, c.ChunkID), c)
}

// BatchRemoveChunkDataPack removes a chunk data pack keyed by chunk ID, in a batch.
// No errors are expected during normal operation, even if no entries are matched.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func BatchRemoveChunkDataPack(chunkID flow.Identifier) func(batch *badger.WriteBatch) error {
	return batchRemove(makePrefix(codeChunkDataPack, chunkID))
}

// RetrieveChunkDataPack retrieves a chunk data pack by chunk ID.
func RetrieveChunkDataPack(chunkID flow.Identifier, c *badgermodel.StoredChunkDataPack) func(*badger.Txn) error {
	return retrieve(makePrefix(codeChunkDataPack, chunkID), c)
}

// RemoveChunkDataPack removes the chunk data pack with the given chunk ID.
func RemoveChunkDataPack(chunkID flow.Identifier) func(*badger.Txn) error {
	return remove(makePrefix(codeChunkDataPack, chunkID))
}
