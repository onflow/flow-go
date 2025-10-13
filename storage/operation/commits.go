package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// IndexStateCommitment indexes a state commitment by the block ID whose execution results in that state.
// The function ensures data integrity by first checking if a commitment already exists for the given block
// and rejecting overwrites with different values. This function is idempotent, i.e. repeated calls with the
// *initially* indexed value are no-ops.
//
// CAUTION:
//   - Confirming that no value is already stored and the subsequent write must be atomic to prevent data corruption.
//     The caller must acquire the [storage.LockInsertOwnReceipt] and hold it until the database write has been committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrDataMismatch] if a *different* state commitment is already indexed for the same block ID
func IndexStateCommitment(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, commit flow.StateCommitment) error {
	if !lctx.HoldsLock(storage.LockInsertOwnReceipt) {
		return fmt.Errorf("cannot index state commitment without holding lock %s", storage.LockInsertOwnReceipt)
	}

	var existingCommit flow.StateCommitment
	err := LookupStateCommitment(rw.GlobalReader(), blockID, &existingCommit) // on happy path, i.e. nothing stored yet, we expect `storage.ErrNotFound`
	if err == nil {                                                           // Value for this key already exists! Need to check for data mismatch:
		if existingCommit == commit {
			return nil // The commit already exists, no need to index again
		}
		return fmt.Errorf("commit for block %v already exists with different value, (existing: %v, new: %v), %w", blockID, existingCommit, commit, storage.ErrDataMismatch)
	} else if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not check existing state commitment: %w", err)
	}

	return UpsertByKey(rw.Writer(), MakePrefix(codeCommit, blockID), commit)
}

// LookupStateCommitment retrieves a state commitment by the block ID whose execution results in that state.
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if no state commitment is indexed for the specified block ID
func LookupStateCommitment(r storage.Reader, blockID flow.Identifier, commit *flow.StateCommitment) error {
	return RetrieveByKey(r, MakePrefix(codeCommit, blockID), commit)
}

// RemoveStateCommitment removes the state commitment by block ID
// CAUTION: this is for recovery purposes only, and should not be used during normal operations!
// It returns nil if no execution result for the given blockID was previously indexed.
// No errors are expected during normal operation.
func RemoveStateCommitment(w storage.Writer, blockID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeCommit, blockID))
}
