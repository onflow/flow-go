package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Results implements persistent storage for execution results.
type Results struct {
	db *badger.DB
}

func NewResults(db *badger.DB) *Results {
	return &Results{
		db: db,
	}
}

func (r *Results) Store(result *flow.ExecutionResult) error {
	return r.db.Update(func(tx *badger.Txn) error {
		err := operation.InsertResult(result)(tx)
		if err != nil {
			return fmt.Errorf("could not insert execution result: %w", err)
		}
		return nil
	})
}

func (r *Results) ByID(resultID flow.Identifier) (*flow.ExecutionResult, error) {
	var result flow.ExecutionResult

	err := r.db.View(func(tx *badger.Txn) error {
		return operation.RetrieveResult(resultID, &result)(tx)
	})
	if err != nil {
		return nil, fmt.Errorf("could not retrieve execution result: %w", err)
	}

	return &result, nil
}
