package stores

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ PersisterStore = (*ScheduledTransactionsStore)(nil)

// ScheduledTransactionsStore handles persisting scheduled transactions
type ScheduledTransactionsStore struct {
	data                  map[flow.Identifier]uint64
	scheduledTransactions storage.ScheduledTransactions
	blockID               flow.Identifier
}

func NewScheduledTransactionsStore(
	data map[flow.Identifier]uint64,
	scheduledTransactions storage.ScheduledTransactions,
	blockID flow.Identifier,
) *ScheduledTransactionsStore {
	return &ScheduledTransactionsStore{
		data:                  data,
		scheduledTransactions: scheduledTransactions,
		blockID:               blockID,
	}
}

// Persist saves and indexes all scheduled transactions for the block as part of the provided database
// batch. The caller must acquire [storage.LockIndexScheduledTransaction] and hold it until the write
// batch has been committed.
// Will return an error if the scheduled transactions are already indexed for the block.
//
// No error returns are expected during normal operations
func (s *ScheduledTransactionsStore) Persist(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
	for txID, scheduledTxID := range s.data {
		err := s.scheduledTransactions.BatchIndex(lctx, s.blockID, txID, scheduledTxID, rw)
		if err != nil {
			return fmt.Errorf("could add index scheduled transaction (%d) %s to batch: %w", scheduledTxID, txID, err)
		}
	}
	return nil
}
