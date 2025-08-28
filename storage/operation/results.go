package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertExecutionResult inserts an Execution Result by its ID.
// Accidental overwrites with inconsistent values are prevented by the collision-resistant hash
// that is used to deriver the key from the value,
// No errors are expected during normal operation.
func InsertExecutionResult(w storage.Writer, result *flow.ExecutionResult) error {
	return UpsertByKey(w, MakePrefix(codeExecutionResult, result.ID()), result)
}

// RetrieveExecutionResult retrieves an Execution Result by its ID.
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no result with the specified `resultID` is known.
func RetrieveExecutionResult(r storage.Reader, resultID flow.Identifier, result *flow.ExecutionResult) error {
	return RetrieveByKey(r, MakePrefix(codeExecutionResult, resultID), result)
}

// IndexExecutionResult indexes the execution node's OWN Execution Result ID keyed by the executed block's ID
//
// CAUTION:
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exits.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption.
//
// TODO: USE LOCK, we want to protect this mapping from accidental overwrites (because the key is not derived from the value via a collision-resistant hash)
func IndexExecutionResult(w storage.Writer, blockID flow.Identifier, resultID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

// LookupExecutionResult finds the execution node's OWN Execution Result for the specified block.
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no result for the `blockID` is known.
func LookupExecutionResult(r storage.Reader, blockID flow.Identifier, resultID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

// ExistExecutionResult checks if the execution node has its OWN Execution Result for the specified block.
// No errors are expected during normal operation.
func ExistExecutionResult(r storage.Reader, blockID flow.Identifier) (bool, error) {
	return KeyExists(r, MakePrefix(codeIndexExecutionResultByBlock, blockID))
}

// RemoveExecutionResultIndex removes execution node's OWN Execution Result for the given blockID.
// It returns nil if the collection does not exist.
// CAUTION: this is for recovery purposes only, and should not be used during normal operations
// No errors are expected during normal operation.
func RemoveExecutionResultIndex(w storage.Writer, blockID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeIndexExecutionResultByBlock, blockID))
}
