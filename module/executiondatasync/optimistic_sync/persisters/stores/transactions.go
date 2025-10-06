package stores

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ PersisterStore = (*TransactionsStore)(nil)

// TransactionsStore handles persisting transactions
type TransactionsStore struct {
	data                  []*flow.TransactionBody
	persistedTransactions storage.Transactions
}

func NewTransactionsStore(
	data []*flow.TransactionBody,
	persistedTransactions storage.Transactions,
) *TransactionsStore {
	return &TransactionsStore{
		data:                  data,
		persistedTransactions: persistedTransactions,
	}
}

// Persist adds transactions to the batch.
//
// No error returns are expected during normal operations
func (t *TransactionsStore) Persist(_ lockctx.Proof, batch storage.ReaderBatchWriter) error {
	for _, transaction := range t.data {
		if err := t.persistedTransactions.BatchStore(transaction, batch); err != nil {
			return fmt.Errorf("could not add transactions to batch: %w", err)
		}
	}

	return nil
}
