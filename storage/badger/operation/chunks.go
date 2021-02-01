package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

func InsertChunk(chunk *flow.Chunk) func(*badger.Txn) error {
	return insert(makePrefix(codeChunk, chunk.ID()), chunk)
}

func RetrieveChunk(chunkID flow.Identifier, chunk *flow.Chunk) func(*badger.Txn) error {
	return retrieve(makePrefix(codeChunk, chunkID), chunk)
}
