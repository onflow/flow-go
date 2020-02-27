package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// InsertExecutionResult inserts a transaction keyed by transaction fingerprint.
func InsertExecutionResult(result *flow.ExecutionResult) func(*badger.Txn) error {
	return insert(makePrefix(codeResult, result.ID()), result)
}

// RetrieveExecutionResult retrieves a transaction by fingerprint.
func RetrieveExecutionResult(resultID flow.Identifier, result *flow.ExecutionResult) func(*badger.Txn) error {
	return retrieve(makePrefix(codeResult, resultID), result)
}
