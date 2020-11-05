package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertExecutionResult inserts an execution result by ID.
func InsertExecutionResult(result *flow.ExecutionResult) func(*badger.Txn) error {
	return insert(makePrefix(codeExecutionResult, result.ID()), result)
}

// UpsertExecutionResult inserts an execution result by ID.
func BatchInsertExecutionResult(result *flow.ExecutionResult) func(batch *badger.WriteBatch) error {
	return batchInsert(makePrefix(codeExecutionResult, result.ID()), result)
}

// RetrieveExecutionResult retrieves a transaction by fingerprint.
func RetrieveExecutionResult(resultID flow.Identifier, result *flow.ExecutionResult) func(*badger.Txn) error {
	return retrieve(makePrefix(codeExecutionResult, resultID), result)
}

// IndexExecutionResult inserts an execution result ID keyed by block ID
func IndexExecutionResult(blockID flow.Identifier, resultID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

// ReindexExecutionResult updates mapping of an execution result ID keyed by block ID
func ReindexExecutionResult(blockID flow.Identifier, resultID flow.Identifier) func(*badger.Txn) error {
	return update(makePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

// BatchIndexExecutionResult inserts an execution result ID keyed by block ID into a batch
func BatchIndexExecutionResult(blockID flow.Identifier, resultID flow.Identifier) func(batch *badger.WriteBatch) error {
	return batchInsert(makePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

// LookupExecutionResult finds execution result ID by block
func LookupExecutionResult(blockID flow.Identifier, resultID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

func RemoveExecutionResult(blockID flow.Identifier, result *flow.ExecutionResult) func(*badger.Txn) error {
	return func(txn *badger.Txn) error {
		// remove index
		err := remove(makePrefix(codeIndexExecutionResultByBlock, blockID))(txn)
		if err != nil {
			return err
		}

		// remove result
		return remove(makePrefix(codeExecutionResult, result.ID()))(txn)
	}
}
