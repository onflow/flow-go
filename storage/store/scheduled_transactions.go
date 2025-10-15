package store

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

var _ storage.ScheduledTransactions = (*ScheduledTransactions)(nil)

// ScheduledTransactions represents persistent storage for scheduled transaction indices.
// Note: no scheduled transactions are stored. Transaction bodies can be generated on-demand using
// the blueprints package. This interface provides access to indices used to lookup the block ID
// that the scheduled transaction was executed in, which allows querying its transaction result.
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
// `txID` is be the TransactionBody.ID of the scheduled transaction.
// `scheduledTxID` is the uint64 id field returned by the system smart contract.
// Requires the lock: [storage.LockIndexScheduledTransaction]
//
// Expected error returns during normal operation:
//   - [storage.ErrAlreadyExists]: if the scheduled transaction is already indexed
func (st *ScheduledTransactions) BatchIndex(lctx lockctx.Proof, blockID flow.Identifier, txID flow.Identifier, scheduledTxID uint64, batch storage.ReaderBatchWriter) error {
	err := operation.IndexScheduledTransactionID(lctx, batch, scheduledTxID, txID)
	if err != nil {
		return fmt.Errorf("failed to batch index scheduled transaction: %w", err)
	}

	err = operation.IndexScheduledTransactionBlockID(lctx, batch, txID, blockID)
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
