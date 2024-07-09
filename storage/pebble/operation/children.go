package operation

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
)

// InsertBlockChildren insert an index to lookup the direct child of a block by its ID
func InsertBlockChildren(blockID flow.Identifier, childrenIDs flow.IdentifierList) func(pebble.Writer) error {
	return insert(makePrefix(codeBlockChildren, blockID), childrenIDs)
}

// UpdateBlockChildren updates the children for a block.
func UpdateBlockChildren(blockID flow.Identifier, childrenIDs flow.IdentifierList) func(pebble.Writer) error {
	return InsertBlockChildren(blockID, childrenIDs)
}

// RetrieveBlockChildren the child block ID by parent block ID
func RetrieveBlockChildren(blockID flow.Identifier, childrenIDs *flow.IdentifierList) func(pebble.Reader) error {
	return retrieve(makePrefix(codeBlockChildren, blockID), childrenIDs)
}
