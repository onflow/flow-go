package operation

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func InsertChunkLocator(w storage.Writer, locator *chunks.Locator) error {
	return UpsertByKey(w, MakePrefix(codeChunk, locator.ID()), locator)
}

func RetrieveChunkLocator(r storage.Reader, locatorID flow.Identifier, locator *chunks.Locator) error {
	return RetrieveByKey(r, MakePrefix(codeChunk, locatorID), locator)
}

func ExistChunkLocator(r storage.Reader, locatorID flow.Identifier) (bool, error) {
	return KeyExists(r, MakePrefix(codeChunk, locatorID))
}
