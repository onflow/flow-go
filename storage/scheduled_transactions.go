package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

type ScheduledTransactionsReader interface {
	// TransactionIDByID returns the transaction ID of the scheduled transaction by its scheduled transaction ID.
	// Note: `scheduledTxID` is the uint64 id field returned by the system smart contract.
	//
	// Expected error returns during normal operation:
	//   - [storage.ErrNotFound]: if no transaction ID is found for the given scheduled transaction ID
	TransactionIDByID(scheduledTxID uint64) (flow.Identifier, error)

	// BlockIDByTransactionID returns the block ID in which the provided system transaction was executed.
	// `txID` is the TransactionBody.ID of the scheduled transaction.
	//
	// Expected error returns during normal operation:
	//   - [storage.ErrNotFound]: if no block ID is found for the given transaction ID
	BlockIDByTransactionID(txID flow.Identifier) (flow.Identifier, error)
}

// ScheduledTransactions represents persistent storage for scheduled transaction indices.
// Note: no scheduled transactions are stored. Transaction bodies can be generated on-demand using
// the blueprints package. This interface provides access to indices used to lookup the block ID
// that the scheduled transaction was executed in, which allows querying its transaction result.
type ScheduledTransactions interface {
	ScheduledTransactionsReader

	// BatchIndex indexes the scheduled transaction by its block ID, transaction ID, and scheduled transaction ID.
	// `txID` is be the TransactionBody.ID of the scheduled transaction.
	// `scheduledTxID` is the uint64 id field returned by the system smart contract.
	// Requires the lock: [storage.LockIndexScheduledTransaction]
	//
	// Expected error returns during normal operation:
	//   - [storage.ErrAlreadyExists]: if the scheduled transaction is already indexed
	BatchIndex(lctx lockctx.Proof, blockID flow.Identifier, txID flow.Identifier, scheduledTxID uint64, batch ReaderBatchWriter) error
}
