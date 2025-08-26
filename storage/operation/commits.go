package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// IndexStateCommitment indexes a state commitment.
//
// State commitments are keyed by the block whose execution results in the state with the given commit.
// It returns storage.ErrDataMismatch if the commit already exists with a different value.
func IndexStateCommitment(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, commit flow.StateCommitment) error {
	if !lctx.HoldsLock(storage.LockInsertOwnReceipt) {
		return fmt.Errorf("cannot index state commitment without holding lock %s", storage.LockInsertOwnReceipt)
	}

	var existingCommit flow.StateCommitment
	err := LookupStateCommitment(rw.GlobalReader(), blockID, &existingCommit)
	if err == nil {
		if existingCommit == commit {
			// The commit already exists, no need to index again
			return nil
		}
		return fmt.Errorf("commit for block %v already exists with different value, (existing: %v, new: %v), %w", blockID,
			existingCommit, commit, storage.ErrDataMismatch)
	} else if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not check existing state commitment: %w", err)
	}

	return UpsertByKey(rw.Writer(), MakePrefix(codeCommit, blockID), commit)
}

// LookupStateCommitment gets a state commitment keyed by block ID
//
// State commitments are keyed by the block whose execution results in the state with the given commit.
func LookupStateCommitment(r storage.Reader, blockID flow.Identifier, commit *flow.StateCommitment) error {
	return RetrieveByKey(r, MakePrefix(codeCommit, blockID), commit)
}

// RemoveStateCommitment removes the state commitment by block ID
func RemoveStateCommitment(w storage.Writer, blockID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeCommit, blockID))
}
