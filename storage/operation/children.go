package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertBlockChildren insert an index to lookup the direct child of a block by its ID
func UpsertBlockChildren(w storage.Writer, blockID flow.Identifier, childrenIDs flow.IdentifierList) error {
	return UpsertByKey(w, MakePrefix(codeBlockChildren, blockID), childrenIDs)
}

// RetrieveBlockChildren the child block ID by parent block ID
func RetrieveBlockChildren(r storage.Reader, blockID flow.Identifier, childrenIDs *flow.IdentifierList) error {
	return RetrieveByKey(r, MakePrefix(codeBlockChildren, blockID), childrenIDs)
}
