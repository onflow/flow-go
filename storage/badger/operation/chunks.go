package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

func InsertChunkLocator(chunkID flow.Identifier, locator *chunks.ChunkLocator) func(*badger.Txn) error {
	return insert(makePrefix(codeChunk, chunkID), locator)
}

func RetrieveChunkLocator(chunkID flow.Identifier, locator *chunks.ChunkLocator) func(*badger.Txn) error {
	return retrieve(makePrefix(codeChunk, chunkID), locator)
}
