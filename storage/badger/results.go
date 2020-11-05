package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
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
	return operation.SkipDuplicates(operation.InsertExecutionResult(result))
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

func (r *ExecutionResults) byBlockID(blockID flow.Identifier) func(*badger.Txn) (*flow.ExecutionResult, error) {
	return func(tx *badger.Txn) (*flow.ExecutionResult, error) {
		var resultID flow.Identifier
		err := operation.LookupExecutionResult(blockID, &resultID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not lookup execution result ID: %w", err)
		}
		return r.byID(resultID)(tx)
	}
}

func (r *ExecutionResults) index(blockID, resultID flow.Identifier) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		err := operation.IndexExecutionResult(blockID, resultID)(tx)
		if err == nil {
			return nil
		}

		if !errors.Is(err, storage.ErrAlreadyExists) {
			return err
		}

		// when trying to index a result for a block, and there is already a result indexed for this block,
		// double check if the indexed result is the same
		var storedResultID flow.Identifier
		err = operation.LookupExecutionResult(blockID, &storedResultID)(tx)
		if err != nil {
			return fmt.Errorf("there is a result stored already, but cannot retrieve it: %w", err)
		}

		if storedResultID != resultID {
			return fmt.Errorf("storing result that is different from the already stored one for block: %v, storing result: %v, stored result: %v. %w",
				blockID, resultID, storedResultID, storage.ErrDataMismatch)
		}

		return nil
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
	err := operation.RetryOnConflict(r.db.Update, r.index(blockID, resultID))
	if err != nil {
		return fmt.Errorf("could not index execution result: %w", err)
	}
	return nil
}

func (r *ExecutionResults) ByBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.byBlockID(blockID)(tx)
}

func (r *ExecutionResults) RemoveByBlockID(blockID flow.Identifier) error {

	result, err := r.ByBlockID(blockID)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil
	}

	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	return r.db.Update(operation.RemoveExecutionResult(blockID, result))
}
