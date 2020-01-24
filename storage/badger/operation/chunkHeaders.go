package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// InsertChunkHeader inserts a chunk header keyed by chunk ID.
func InsertChunkHeader(c *flow.ChunkHeader) func(*badger.Txn) error {
	return insert(makePrefix(codeChunkHeader, c.ChunkID), c)
}

// RetrieveChunkHeader retrieves a chunk header by chunk ID.
func RetrieveChunkHeader(chunkID flow.Identifier, c *flow.ChunkHeader) func(*badger.Txn) error {
	return retrieve(makePrefix(codeChunkHeader, chunkID), c)
}

// RemoveChunkHeader removes the chunk header with the given chunk ID.
func RemoveChunkHeader(txID flow.Identifier) func(*badger.Txn) error {
	return remove(makePrefix(codeChunkHeader, txID))
}
