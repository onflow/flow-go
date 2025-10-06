package store

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

var _ storage.ScheduledTransactions = (*ScheduledTransactions)(nil)

type ScheduledTransactions struct {
	db           storage.DB
	cacheTxID    *Cache[uint64, flow.Identifier]
	cacheBlockID *Cache[flow.Identifier, flow.Identifier]
}

func NewScheduledTransactions(collector module.CacheMetrics, db storage.DB, cacheSize uint) *ScheduledTransactions {
	retrieveTxID := func(r storage.Reader, scheduledTxID uint64) (flow.Identifier, error) {
		var txID flow.Identifier
		err := operation.RetrieveTransactionIDByScheduledTransactionID(r, scheduledTxID, &txID)
		if err != nil {
			return flow.Identifier{}, err
		}
		return txID, nil
	}

	retrieveBlockID := func(r storage.Reader, txID flow.Identifier) (flow.Identifier, error) {
		var blockID flow.Identifier
		err := operation.RetrieveBlockIDByScheduledTransactionID(r, txID, &blockID)
		if err != nil {
			return flow.Identifier{}, err
		}
		return blockID, nil
	}

	return &ScheduledTransactions{
		db: db,
		cacheTxID: newCache(collector, metrics.ResourceScheduledTransactionsIndices,
			withLimit[uint64, flow.Identifier](cacheSize),
			withStore(noopStore[uint64, flow.Identifier]),
			withRetrieve(retrieveTxID),
		),
		cacheBlockID: newCache(collector, metrics.ResourceScheduledTransactionsIndices,
			withLimit[flow.Identifier, flow.Identifier](cacheSize),
			withStore(noopStore[flow.Identifier, flow.Identifier]),
			withRetrieve(retrieveBlockID),
		),
	}
}

// BatchIndex indexes the scheduled transaction by its block ID, transaction ID, and scheduled transaction ID.
// `scheduledTxID` is the uint64 id field returned by the system smart contract.
// `txID` is be the TransactionBody.ID of the scheduled transaction.
//
// No errors are expected during normal operation.
func (st *ScheduledTransactions) BatchIndex(blockID flow.Identifier, txID flow.Identifier, scheduledTxID uint64, batch storage.ReaderBatchWriter) error {
	writer := batch.Writer()

	err := operation.BatchIndexScheduledTransactionID(writer, scheduledTxID, txID)
	if err != nil {
		return fmt.Errorf("failed to batch index scheduled transaction: %w", err)
	}

	err = operation.BatchIndexScheduledTransactionBlockID(writer, txID, blockID)
	if err != nil {
		return fmt.Errorf("failed to batch index scheduled transaction block ID: %w", err)
	}

	storage.OnCommitSucceed(batch, func() {
		st.cacheTxID.Insert(scheduledTxID, txID)
		st.cacheBlockID.Insert(txID, blockID)
	})

	return nil
}

// TransactionIDByID returns the transaction ID of the scheduled transaction by its scheduled transaction ID.
// `scheduledTxID` is the uint64 `id` field returned by the system smart contract.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if no transaction ID is found for the given scheduled transaction ID
func (st *ScheduledTransactions) TransactionIDByID(scheduledTxID uint64) (flow.Identifier, error) {
	return st.cacheTxID.Get(st.db.Reader(), scheduledTxID)
}

// BlockIDByTransactionID returns the block ID in which the provided system transaction was executed.
// `txID` must be the TransactionBody.ID of the scheduled transaction.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if no block ID is found for the given transaction ID
func (st *ScheduledTransactions) BlockIDByTransactionID(txID flow.Identifier) (flow.Identifier, error) {
	return st.cacheBlockID.Get(st.db.Reader(), txID)
}
