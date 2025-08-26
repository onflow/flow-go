package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// UpsertBlockChildren insert an index to lookup the direct child of a block by its ID
func UpsertBlockChildren(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, childrenIDs flow.IdentifierList) error {
	held := lctx.HoldsLock(storage.LockInsertBlock) || lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock)
	if !held {
		return fmt.Errorf("missing required lock: %s or %s", storage.LockInsertBlock, storage.LockInsertOrFinalizeClusterBlock)
	}

	return UpsertByKey(w, MakePrefix(codeBlockChildren, blockID), childrenIDs)
}

// RetrieveBlockChildren the child block ID by parent block ID
func RetrieveBlockChildren(r storage.Reader, blockID flow.Identifier, childrenIDs *flow.IdentifierList) error {
	return RetrieveByKey(r, MakePrefix(codeBlockChildren, blockID), childrenIDs)
}
