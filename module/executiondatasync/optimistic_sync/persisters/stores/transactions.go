package stores

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
)

var _ PersisterStore = (*TransactionsStore)(nil)

// TransactionsStore handles persisting transactions
type TransactionsStore struct {
	inMemoryTransactions  *unsynchronized.Transactions
	persistedTransactions storage.Transactions
}

func NewTransactionsStore(
	inMemoryTransactions *unsynchronized.Transactions,
	persistedTransactions storage.Transactions,
) *TransactionsStore {
	return &TransactionsStore{
		inMemoryTransactions:  inMemoryTransactions,
		persistedTransactions: persistedTransactions,
	}
}

// Persist adds transactions to the batch.
// No errors are expected during normal operations
func (t *TransactionsStore) Persist(lctx lockctx.Proof, batch storage.ReaderBatchWriter) error {
	for _, transaction := range t.inMemoryTransactions.Data() {
		if err := t.persistedTransactions.BatchStore(&transaction, batch); err != nil {
			return fmt.Errorf("could not add transactions to batch: %w", err)
		}
	}

	return nil
}
