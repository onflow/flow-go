package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// InsertResult inserts an execution result by ID.
func InsertResult(result *flow.ExecutionResult) func(*badger.Txn) error {
	return insert(makePrefix(codeResult, result.ID()), result)
}

// RetrieveResult retrieves an execution result by ID.
func RetrieveResult(resultID flow.Identifier, result *flow.ExecutionResult) func(*badger.Txn) error {
	return retrieve(makePrefix(codeResult, resultID), result)
}
