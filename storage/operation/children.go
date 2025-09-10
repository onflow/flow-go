package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// UpsertBlockChildren updates the children of the specified parent block ID.
//
// CAUTION:
//   - The caller must acquire either the lock [storage.LockInsertBlock] or [storage.LockInsertOrFinalizeClusterBlock] (but not both)
//     and hold it until the database write has been committed.
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exits.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption. The lock proof
//     serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
//     ATOMICALLY with this write operation.
//
// No errors are expected during normal operation.
func UpsertBlockChildren(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, childrenIDs flow.IdentifierList) error {
	held := lctx.HoldsLock(storage.LockInsertBlock) || lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock)
	if !held {
		return fmt.Errorf("missing required lock: %s or %s", storage.LockInsertBlock, storage.LockInsertOrFinalizeClusterBlock)
	}

	return UpsertByKey(w, MakePrefix(codeBlockChildren, blockID), childrenIDs)
}

// RetrieveBlockChildren retrieves the list of child block IDs for the specified parent block.
// For every known block (at or above the root block height), this index should be populated.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if `blockID` does not refer to a known block
func RetrieveBlockChildren(r storage.Reader, blockID flow.Identifier, childrenIDs *flow.IdentifierList) error {
	return RetrieveByKey(r, MakePrefix(codeBlockChildren, blockID), childrenIDs)
}
