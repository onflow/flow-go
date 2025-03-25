package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertExecutionResult inserts an execution result by ID.
func InsertExecutionResult(w storage.Writer, result *flow.ExecutionResult) error {
	return UpsertByKey(w, MakePrefix(codeExecutionResult, result.ID()), result)
}

// RetrieveExecutionResult retrieves a transaction by fingerprint.
func RetrieveExecutionResult(r storage.Reader, resultID flow.Identifier, result *flow.ExecutionResult) error {
	return RetrieveByKey(r, MakePrefix(codeExecutionResult, resultID), result)
}

// IndexExecutionResult inserts an execution result ID keyed by block ID
func IndexExecutionResult(w storage.Writer, blockID flow.Identifier, resultID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

// LookupExecutionResult finds execution result ID by block
func LookupExecutionResult(r storage.Reader, blockID flow.Identifier, resultID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

func ExistExecutionResult(r storage.Reader, blockID flow.Identifier) (bool, error) {
	return KeyExists(r, MakePrefix(codeIndexExecutionResultByBlock, blockID))
}

// RemoveExecutionResultIndex removes execution result indexed by the given blockID
func RemoveExecutionResultIndex(w storage.Writer, blockID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeIndexExecutionResultByBlock, blockID))
}
