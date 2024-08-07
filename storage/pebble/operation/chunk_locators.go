package operation

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

func InsertChunkLocator(locator *chunks.Locator) func(pebble.Writer) error {
	return insert(makePrefix(codeChunk, locator.ID()), locator)
}

func RetrieveChunkLocator(locatorID flow.Identifier, locator *chunks.Locator) func(pebble.Reader) error {
	return retrieve(makePrefix(codeChunk, locatorID), locator)
}

func HasChunkLocator(locatorID flow.Identifier, exist *bool) func(pebble.Reader) error {
	return exists(makePrefix(codeChunk, locatorID), exist)
}
