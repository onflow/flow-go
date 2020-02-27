package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// InsertChunkDataPack inserts a chunk data pack keyed by chunk ID.
func InsertChunkDataPack(c *flow.ChunkDataPack) func(*badger.Txn) error {
	return insert(makePrefix(codeChunkHeader, c.ChunkID), c)
}

// RetrieveChunkDataPack retrieves a chunk data pack by chunk ID.
func RetrieveChunkDataPack(chunkID flow.Identifier, c *flow.ChunkDataPack) func(*badger.Txn) error {
	return retrieve(makePrefix(codeChunkHeader, chunkID), c)
}

// RemoveChunkDataPack removes the chunk data pack with the given chunk ID.
func RemoveChunkDataPack(txID flow.Identifier) func(*badger.Txn) error {
	return remove(makePrefix(codeChunkHeader, txID))
}
