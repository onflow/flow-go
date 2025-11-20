package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertExecutionResult inserts a [flow.ExecutionResult] into the storage, keyed by its ID.
//
// CAUTION: The caller must ensure `resultID` is a collision-resistant hash of the provided `result`!
// This method silently overrides existing data, which is safe only if for the same key, we always
// write the same value.
//
// No error returns are expected during normal operations.
func InsertExecutionResult(w storage.Writer, resultID flow.Identifier, result *flow.ExecutionResult) error {
	return UpsertByKey(w, MakePrefix(codeExecutionResult, resultID), result)
}

// RetrieveExecutionResult retrieves an Execution Result by its ID.
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if no result with the specified `resultID` is known.
func RetrieveExecutionResult(r storage.Reader, resultID flow.Identifier, result *flow.ExecutionResult) error {
	return RetrieveByKey(r, MakePrefix(codeExecutionResult, resultID), result)
}

// IndexTrustedExecutionResult indexes the result for the given block.
// It is used by the following scenarios:
// 1. Execution Node indexes its own executed block's result when finish executing a block
// 2. Execution Node indexes the sealed root block's result during bootstrapping
//
// The caller must acquire [storage.LockIndexExecutionResult]
//
// It returns [storage.ErrDataMismatch] if there is already an indexed result for the given blockID,
// but it is different from the given resultID.
func IndexTrustedExecutionResult(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, resultID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockIndexExecutionResult) {
		return fmt.Errorf("missing require locks: %s", storage.LockIndexExecutionResult)
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
	} else if !errors.Is(err, storage.ErrNotFound) {
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
