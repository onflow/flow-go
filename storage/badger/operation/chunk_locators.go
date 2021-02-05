package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

func InsertChunkLocator(locator *chunks.Locator) func(*badger.Txn) error {
	return insert(makePrefix(codeChunk, locator.ID()), locator)
}

func RetrieveChunkLocator(locatorID flow.Identifier, locator *chunks.Locator) func(*badger.Txn) error {
	return retrieve(makePrefix(codeChunk, locatorID), locator)
}
