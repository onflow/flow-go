package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type TransactionResults struct {
	db *badger.DB
}

func NewTransactionResults(db *badger.DB) *TransactionResults {
	return &TransactionResults{
		db: db,
	}
}

// Store will store the transaction result for the given block ID
func (tr *TransactionResults) Store(blockID flow.Identifier, transactionResult *flow.TransactionResult) error {
	return tr.db.Update(func(btx *badger.Txn) error {
		err := operation.InsertTransactionResult(blockID, transactionResult)(btx)
		if err != nil {
			return fmt.Errorf("could not insert event: %w", err)
		}

		return nil
	})
}

// ByBlockIDTransactionID returns the runtime transaction result for the given block ID and transaction ID
func (tr *TransactionResults) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) (*flow.TransactionResult, error) {

	var txResult flow.TransactionResult
	err := tr.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveTransactionResult(blockID, txID, &txResult)(btx)
		return handleError(err, flow.TransactionResult{})
	})

	if err != nil {
		return nil, err
	}

	return &txResult, nil
}
