package operation

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func InsertChunkLocator(locator *chunks.Locator) func(storage.Writer) error {
	return insertW(makePrefix(codeChunk, locator.ID()), locator)
}

func RetrieveChunkLocator(locatorID flow.Identifier, locator *chunks.Locator) func(storage.Reader) error {
	return retrieveR(makePrefix(codeChunk, locatorID), locator)
}

func HasChunkLocator(locatorID flow.Identifier, exist *bool) func(storage.Reader) error {
	return existsR(makePrefix(codeChunk, locatorID), exist)
}
