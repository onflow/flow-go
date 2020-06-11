package operation

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

// InsertExecutionResult inserts an execution result by ID.
func InsertExecutionResult(result *flow.ExecutionResult) func(*badger.Txn) error {
	return insert(makePrefix(codeExecutionResult, result.ID()), result)
}

// RetrieveExecutionResult retrieves a transaction by fingerprint.
func RetrieveExecutionResult(resultID flow.Identifier, result *flow.ExecutionResult) func(*badger.Txn) error {
	return retrieve(makePrefix(codeExecutionResult, resultID), result)
}

// IndexExecutionResult inserts an execution result ID keyed by block ID
func IndexExecutionResult(blockID flow.Identifier, resultID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

// LookupExecutionResult finds execution result ID by block
func LookupExecutionResult(blockID flow.Identifier, resultID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

// IndexExecutionResultsByBlockID performs a database migration that indexes
// all execution receipts in the database by their block ID.
//
//TODO find a better home for this
// Ideally `common.go` should live in an `internal` package and be exported so
// we can better organize the storage layer code. Then this could live in ie. a
// migrations package, but still import the relevant shared code from `common.go`.
func IndexExecutionResultsByBlockID(db *badger.DB) error {

	// we split the migration into 256 batches, one transaction each, to avoid
	// hitting the max transaction size
	batchNumber := uint8(0)

	for {
		err := db.Update(func(tx *badger.Txn) error {
			iter := func() (checkFunc, createFunc, handleFunc) {
				check := func(key []byte) bool {
					return true
				}

				var result flow.ExecutionResult
				create := func() interface{} {
					return &result
				}

				handle := func() error {
					err := IndexExecutionResult(result.BlockID, result.ID())(tx)
					// skip already indexed entities
					if errors.Is(err, storage.ErrAlreadyExists) {
						return nil
					}
					if err != nil {
						return fmt.Errorf("could not index execution result: %w", err)
					}
					return nil
				}

				return check, create, handle
			}

			return traverse(makePrefix(codeExecutionResult, batchNumber), iter)(tx)
		})
		if err != nil {
			return fmt.Errorf("ER index migration failed (batch=%d): %w", batchNumber, err)
		}

		if batchNumber == 255 {
			return nil
		}
		batchNumber++
	}
}
