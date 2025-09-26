package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertProposalSignature inserts a proposal signature by block ID.
//
// CAUTION:
//   - The caller must acquire either the lock [storage.LockInsertBlock] or [storage.LockInsertOrFinalizeClusterBlock] (but not both)
//     and hold it until the database write has been committed.
//   - Since the signature is indexed by block ID, and the caller holding this lock has done the
//     necessary checks to ensure that the block ID is new, and that no signature exists for it yet,
//     so this function will not check for existing entries when inserting.
//
// No errors are expected during normal operation.
func InsertProposalSignature(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, sig *[]byte) error {
	held := lctx.HoldsLock(storage.LockInsertBlock) || lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock)
	if !held {
		return fmt.Errorf("missing required lock: %s or %s", storage.LockInsertBlock, storage.LockInsertOrFinalizeClusterBlock)
	}

	return UpsertByKey(w, MakePrefix(codeBlockIDToProposalSignature, blockID), sig)
}

// RetrieveProposalSignature retrieves a proposal signature by blockID.
// Returns storage.ErrNotFound if no proposal signature is stored for the block.
func RetrieveProposalSignature(r storage.Reader, blockID flow.Identifier, sig *[]byte) error {
	return RetrieveByKey(r, MakePrefix(codeBlockIDToProposalSignature, blockID), sig)
}
