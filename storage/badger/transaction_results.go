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
	err := operation.RetryOnConflict(tr.db.Update, operation.SkipDuplicates(operation.InsertTransactionResult(blockID, transactionResult)))
	if err != nil {
		return fmt.Errorf("could not insert transaction result: %w", err)
	}
	return nil
}

// ByBlockIDTransactionID returns the runtime transaction result for the given block ID and transaction ID
func (tr *TransactionResults) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) (*flow.TransactionResult, error) {

	var txResult flow.TransactionResult
	err := tr.db.View(operation.RetrieveTransactionResult(blockID, txID, &txResult))
	if err != nil {
		return nil, handleError(err, flow.TransactionResult{})
	}

	return &txResult, nil
}
