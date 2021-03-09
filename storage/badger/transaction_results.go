package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
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

// BatchStore will store the transaction results for the given block ID in a batch
func (tr *TransactionResults) BatchStore(blockID flow.Identifier, transactionResults []flow.TransactionResult, batch storage.BatchStorage) error {

	if writeBatch, ok := batch.(*badger.WriteBatch); ok {
		for _, result := range transactionResults {
			err := operation.BatchInsertTransactionResult(blockID, &result)(writeBatch)
			if err != nil {
				return fmt.Errorf("cannot batch insert tx result: %w", err)
			}
		}
		return nil
	}
	return fmt.Errorf("unsupported BatchStore type %T", batch)
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
