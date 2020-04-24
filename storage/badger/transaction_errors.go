package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type TransactionErrors struct {
	db *badger.DB
}

func NewTransactionErrors(db *badger.DB) *TransactionErrors {
	return &TransactionErrors{
		db: db,
	}
}

// Store will store runtime cadence error for the given block ID
func (e *TransactionErrors) Store(blockID flow.Identifier, transactionError *flow.TransactionError) error {
	return e.db.Update(func(btx *badger.Txn) error {
		err := operation.InsertTransactionError(blockID, transactionError)(btx)
		if err != nil {
			return fmt.Errorf("could not insert event: %w", err)
		}

		return nil
	})
}

// ByBlockIDTransactionID returns the runtime cadence error for the given block ID and transaction ID
func (e *TransactionErrors) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) (*flow.TransactionError, error) {

	var txError flow.TransactionError
	err := e.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveTransactionError(blockID, txID, &txError)(btx)
		return handleError(err, flow.TransactionError{})
	})

	if err != nil {
		return nil, err
	}

	return &txError, nil
}
