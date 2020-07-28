package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// InsertChunkDataPack inserts a chunk data pack keyed by chunk ID.
func InsertChunkDataPack(c *flow.ChunkDataPack) func(*badger.Txn) error {
	return insert(makePrefix(codeChunkDataPack, c.ChunkID), c)
}

// RetrieveChunkDataPack retrieves a chunk data pack by chunk ID.
func RetrieveChunkDataPack(chunkID flow.Identifier, c *flow.ChunkDataPack) func(*badger.Txn) error {
	return retrieve(makePrefix(codeChunkDataPack, chunkID), c)
}

// RemoveChunkDataPack removes the chunk data pack with the given chunk ID.
func RemoveChunkDataPack(chunkID flow.Identifier) func(*badger.Txn) error {
	return remove(makePrefix(codeChunkDataPack, chunkID))
}

// FindHeaders iterates through all items, calling `onEach` on each, which
// return boolean indicating if next record should be server
func IterateChunkDataPacks(onEach func(header *flow.ChunkDataPack) bool) func(*badger.Txn) error {
	return traverse(makePrefix(codeExecutionResult), func() (checkFunc, createFunc, handleFunc) {
		check := func(key []byte) bool {
			return true
		}
		var val flow.ChunkDataPack
		create := func() interface{} {
			return &val
		}
		handle := func() error {
			if !onEach(&val) {
				return EndIterationError
			}
			return nil
		}
		return check, create, handle
	})
}
