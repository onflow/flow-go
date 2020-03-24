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

func (r *ExecutionResults) Store(result *flow.ExecutionResult) error {
	return r.db.Update(func(tx *badger.Txn) error {
		err := operation.InsertExecutionResult(result)(tx)
		if err != nil {
			return fmt.Errorf("could not insert execution result: %w", err)
		}
		return nil
	})
}

func (r *ExecutionResults) Index(blockID flow.Identifier, resultID flow.Identifier) error {
	return r.db.Update(func(tx *badger.Txn) error {
		err := operation.IndexExecutionResult(blockID, resultID)(tx)
		if err != nil {
			return fmt.Errorf("could not index execution result: %w", err)
		}
		return nil
	})
}

func (r *ExecutionResults) Lookup(blockID flow.Identifier) (flow.Identifier, error) {
	var result flow.Identifier
	err := r.db.View(func(tx *badger.Txn) error {
		return operation.LookupExecutionResult(blockID, &result)(tx)
	})
	if err != nil {
		return flow.Identifier{}, fmt.Errorf("coulnd not lookup execution result: %w", err)
	}

	return result, nil
}

func (r *ExecutionResults) ByID(resultID flow.Identifier) (*flow.ExecutionResult, error) {
	var result flow.ExecutionResult

	err := r.db.View(func(tx *badger.Txn) error {
		return operation.RetrieveExecutionResult(resultID, &result)(tx)
	})
	if err != nil {
		return nil, fmt.Errorf("could not retrieve execution result: %w", err)
	}

	return &result, nil
}
