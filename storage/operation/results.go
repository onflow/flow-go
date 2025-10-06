package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertExecutionResult inserts a [flow.ExecutionResult] into the storage, keyed by its ID.
//
// If the result already exists, it will be overwritten. Note that here, the key (result ID) is derived
// from the value (result) via a collision-resistant hash function. Hence, unchecked overwrites pose no risk
// of data corruption, because for the same key, we expect the same value.
//
// No errors are expected during normal operation.
func InsertExecutionResult(w storage.Writer, resultID flow.Identifier, result *flow.ExecutionResult) error {
	return UpsertByKey(w, MakePrefix(codeExecutionResult, resultID), result)
}

// RetrieveExecutionResult retrieves an Execution Result by its ID.
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no result with the specified `resultID` is known.
func RetrieveExecutionResult(r storage.Reader, resultID flow.Identifier, result *flow.ExecutionResult) error {
	return RetrieveByKey(r, MakePrefix(codeExecutionResult, resultID), result)
}

// IndexOwnOrSealedExecutionResult indexes the result of the given block.
// It is used by EN to index the result of a block to continue executing subsequent blocks.
// The caller must acquire either [storage.LockInsertOwnReceipt] or [storage.LockBootstrapping] or [storage.LockIndexFinalizedBlock]
//
// No errors are expected during normal operation.
func IndexOwnOrSealedExecutionResult(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, resultID flow.Identifier) error {
	held := lctx.HoldsLock(storage.LockInsertOwnReceipt) ||
		// during bootstrapping, we index the sealed root block or the spork root block, which is not
		// produced by the node itself, but we still need to index its execution result to be able to
		// execute next block
		lctx.HoldsLock(storage.LockBootstrapping) ||
		lctx.HoldsLock(storage.LockIndexFinalizedBlock)
	if !held {
		return fmt.Errorf("missing require locks: %s or %s", storage.LockInsertOwnReceipt, storage.LockBootstrapping)
	}

	key := MakePrefix(codeIndexExecutionResultByBlock, blockID)
	var existing flow.Identifier
	err := RetrieveByKey(rw.GlobalReader(), key, &existing)
	if err == nil {
		if existing != resultID {
			return fmt.Errorf("storing result that is different from the already stored one for block: %v, storing result: %v, stored result: %v. %w",
				blockID, resultID, existing, storage.ErrDataMismatch)
		}
		// if the result is the same, we don't need to index it again
		return nil
	} else if err != storage.ErrNotFound {
		return fmt.Errorf("could not check if execution result exists: %w", err)
	}

	// if the result is not indexed, we can index it
	return UpsertByKey(rw.Writer(), key, resultID)
}

// LookupExecutionResult retrieves the Execution Node's OWN Execution Result ID for the specified block.
// Intended for Execution Node only. For every block executed by this node, this index should be populated.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if `blockID` does not refer to a block executed by this node
func LookupExecutionResult(r storage.Reader, blockID flow.Identifier, resultID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

// RemoveExecutionResultIndex removes Execution Node's OWN Execution Result for the given blockID.
// CAUTION: this is for recovery purposes only, and should not be used during normal operations
// It returns nil if the collection does not exist.
// No errors are expected during normal operation.
func RemoveExecutionResultIndex(w storage.Writer, blockID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeIndexExecutionResultByBlock, blockID))
}
