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
//   - OVERWRITES existing data (potential for data corruption):
//     The lock proof serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK
//     is done elsewhere ATOMICALLY with this write operation. It is intended that this function is called only for new
//     blocks, i.e. no signature was previously persisted for it.
//
// No errors are expected during normal operation.
func InsertProposalSignature(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, sig *[]byte) error {
	held, errStr := storage.HeldOneLock(lctx, storage.LockInsertBlock, storage.LockInsertOrFinalizeClusterBlock)
	if !held {
		return fmt.Errorf("%s", errStr)
	}

	return UpsertByKey(w, MakePrefix(codeBlockIDToProposalSignature, blockID), sig)
}

// RetrieveProposalSignature retrieves a proposal signature by blockID.
// Returns storage.ErrNotFound if no proposal signature is stored for the block.
func RetrieveProposalSignature(r storage.Reader, blockID flow.Identifier, sig *[]byte) error {
	return RetrieveByKey(r, MakePrefix(codeBlockIDToProposalSignature, blockID), sig)
}
