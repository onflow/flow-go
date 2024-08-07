package operation

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
)

// InsertExecutionResult inserts an execution result by ID.
func InsertExecutionResult(result *flow.ExecutionResult) func(pebble.Writer) error {
	return insert(makePrefix(codeExecutionResult, result.ID()), result)
}

// RetrieveExecutionResult retrieves a transaction by fingerprint.
func RetrieveExecutionResult(resultID flow.Identifier, result *flow.ExecutionResult) func(pebble.Reader) error {
	return retrieve(makePrefix(codeExecutionResult, resultID), result)
}

// IndexExecutionResult inserts an execution result ID keyed by block ID
func IndexExecutionResult(blockID flow.Identifier, resultID flow.Identifier) func(pebble.Writer) error {
	return insert(makePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

// LookupExecutionResult finds execution result ID by block
func LookupExecutionResult(blockID flow.Identifier, resultID *flow.Identifier) func(pebble.Reader) error {
	return retrieve(makePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

// RemoveExecutionResultIndex removes execution result indexed by the given blockID
func RemoveExecutionResultIndex(blockID flow.Identifier) func(pebble.Writer) error {
	return remove(makePrefix(codeIndexExecutionResultByBlock, blockID))
}
