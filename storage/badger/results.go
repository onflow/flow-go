package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// ExecutionResults implements persistent storage for execution results.
type ExecutionResults struct {
	db *badger.DB
}

func NewExecutionResults(db *badger.DB) *ExecutionResults {
	return &ExecutionResults{
		db: db,
	}
}

func (r *ExecutionResults) store(result *flow.ExecutionResult) func(*badger.Txn) error {
	return operation.InsertExecutionResult(result)
}

func (r *ExecutionResults) byID(resultID flow.Identifier) func(*badger.Txn) (*flow.ExecutionResult, error) {
	return func(tx *badger.Txn) (*flow.ExecutionResult, error) {
		var result flow.ExecutionResult
		err := operation.RetrieveExecutionResult(resultID, &result)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve execution result: %w", err)
		}

		return &result, nil
	}
}

func (r *ExecutionResults) Store(result *flow.ExecutionResult) error {
	return operation.RetryOnConflict(r.db.Update, r.store(result))
}

func (r *ExecutionResults) ByID(resultID flow.Identifier) (*flow.ExecutionResult, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.byID(resultID)(tx)
}

func (r *ExecutionResults) Index(blockID flow.Identifier, resultID flow.Identifier) error {
	err := operation.RetryOnConflict(r.db.Update, operation.IndexExecutionResult(blockID, resultID))
	if err != nil {
		return fmt.Errorf("could not index execution result: %w", err)
	}
	return nil
}

func (r *ExecutionResults) ByBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error) {
	var resultID flow.Identifier
	err := r.db.View(operation.LookupExecutionResult(blockID, &resultID))
	if err != nil {
		return nil, fmt.Errorf("could not lookup execution result ID: %w", err)
	}
	return r.ByID(resultID)
}
