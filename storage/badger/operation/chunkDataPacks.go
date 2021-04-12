package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertChunkDataPack inserts a chunk data pack keyed by chunk ID.
func InsertChunkDataPack(c *flow.ChunkDataPack) func(*badger.Txn) error {
	return insert(makePrefix(codeChunkDataPack, c.ChunkID), c)
}

// BatchInsertChunkDataPack upserts a chunk data pack keyed by chunk ID into a batch
func BatchInsertChunkDataPack(c *flow.ChunkDataPack) func(batch *badger.WriteBatch) error {
	return batchInsert(makePrefix(codeChunkDataPack, c.ChunkID), c)
}

// RetrieveChunkDataPack retrieves a chunk data pack by chunk ID.
func RetrieveChunkDataPack(chunkID flow.Identifier, c *flow.ChunkDataPack) func(*badger.Txn) error {
	return retrieve(makePrefix(codeChunkDataPack, chunkID), c)
}

// RemoveChunkDataPack removes the chunk data pack with the given chunk ID.
func RemoveChunkDataPack(chunkID flow.Identifier) func(*badger.Txn) error {
	return remove(makePrefix(codeChunkDataPack, chunkID))
}
