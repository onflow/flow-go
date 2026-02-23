package storage

import (
	"github.com/jordanschalm/lockctx"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// ScheduledTransactionsIndexReader provides read access to the scheduled transactions index.
//
// All methods are safe for concurrent access.
type ScheduledTransactionsIndexReader interface {
	// ByID returns the scheduled transaction with the given scheduler-assigned ID.
	//
	// Expected error returns during normal operation:
	//   - [ErrNotFound]: if no scheduled transaction with the given ID exists
	ByID(id uint64) (accessmodel.ScheduledTransaction, error)

	// ByAddress retrieves scheduled transactions for an account using cursor-based pagination.
	// Results are returned in descending order (highest ID first).
	//
	// `limit` specifies the maximum number of results per page.
	//
	// `cursor` is a pointer to a [accessmodel.ScheduledTransactionCursor]:
	//   - nil means start from the highest indexed ID (first page)
	//   - non-nil means resume after the cursor position (subsequent pages)
	//
	// `filter` is an optional filter applied before calculating the limit. If nil, all
	// transactions are returned. The same filter must be applied across all pages.
	//
	// Expected error returns during normal operation:
	//   - [ErrNotBootstrapped]: if the index has not been initialized
	//   - [ErrInvalidQuery]: if limit is invalid
	ByAddress(
		account flow.Address,
		limit uint32,
		cursor *accessmodel.ScheduledTransactionCursor,
		filter IndexFilter[*accessmodel.ScheduledTransaction],
	) (accessmodel.ScheduledTransactionsPage, error)

	// All retrieves all scheduled transactions using cursor-based pagination.
	// Results are returned in descending order (highest ID first).
	//
	// `limit` specifies the maximum number of results per page.
	//
	// `cursor` is a pointer to a [accessmodel.ScheduledTransactionCursor]:
	//   - nil means start from the highest indexed ID (first page)
	//   - non-nil means resume after the cursor position (subsequent pages)
	//
	// `filter` is an optional filter applied before calculating the limit. If nil, all
	// transactions are returned. The same filter must be applied across all pages.
	//
	// Expected error returns during normal operation:
	//   - [ErrNotBootstrapped]: if the index has not been initialized
	//   - [ErrInvalidQuery]: if limit is invalid
	All(
		limit uint32,
		cursor *accessmodel.ScheduledTransactionCursor,
		filter IndexFilter[*accessmodel.ScheduledTransaction],
	) (accessmodel.ScheduledTransactionsPage, error)
}

// ScheduledTransactionsIndexRangeReader provides access to the range of indexed heights.
//
// All methods are safe for concurrent access.
type ScheduledTransactionsIndexRangeReader interface {
	// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
	FirstIndexedHeight() uint64

	// LatestIndexedHeight returns the latest block height that has been indexed.
	LatestIndexedHeight() uint64
}

// ScheduledTransactionsIndexWriter provides write access to the scheduled transactions index.
//
// NOT CONCURRENTLY SAFE.
type ScheduledTransactionsIndexWriter interface {
	// Store indexes all new scheduled transactions from the given block and advances
	// the latest indexed height to blockHeight. Must be called with consecutive heights.
	// The caller must hold the [LockIndexScheduledTransactionsIndex] lock until the batch
	// is committed.
	//
	// Expected error returns during normal operation:
	//   - [ErrAlreadyExists]: if blockHeight is already indexed
	Store(
		lctx lockctx.Proof,
		rw ReaderBatchWriter,
		blockHeight uint64,
		scheduledTxs []accessmodel.ScheduledTransaction,
	) error

	// Executed updates the scheduled transaction's status to Executed and records the
	// final execution effort and the ID of the transaction that emitted the Executed event.
	// The caller must hold the [LockIndexScheduledTransactionsIndex] lock until the batch
	// is committed.
	//
	// Expected error returns during normal operation:
	//   - [ErrNotFound]: if no scheduled transaction with the given ID exists
	//   - [ErrInvalidStatusTransition]: if the transaction is already Executed, Cancelled, or Failed
	Executed(
		lctx lockctx.Proof,
		rw ReaderBatchWriter,
		scheduledTxID uint64,
		executionEffort uint64,
		transactionID flow.Identifier,
	) error

	// Cancelled updates the scheduled transaction's status to Cancelled and records the
	// fee amounts and the ID of the transaction that emitted the Canceled event.
	// The caller must hold the [LockIndexScheduledTransactionsIndex] lock until the batch
	// is committed.
	//
	// Expected error returns during normal operation:
	//   - [ErrNotFound]: if no scheduled transaction with the given ID exists
	//   - [ErrInvalidStatusTransition]: if the transaction is already Executed, Cancelled, or Failed
	Cancelled(
		lctx lockctx.Proof,
		rw ReaderBatchWriter,
		scheduledTxID uint64,
		feesReturned uint64,
		feesDeducted uint64,
		transactionID flow.Identifier,
	) error

	// Failed updates the transaction's status to Failed and records the ID of the executor
	// transaction that attempted (and failed) to execute the scheduled transaction.
	// The caller must hold the [LockIndexScheduledTransactionsIndex] lock until committed.
	//
	// Expected error returns during normal operation:
	//   - [ErrNotFound]: if no entry with the given ID exists
	//   - [ErrInvalidStatusTransition]: if the transaction is already Executed, Cancelled, or Failed
	Failed(
		lctx lockctx.Proof,
		rw ReaderBatchWriter,
		scheduledTxID uint64,
		transactionID flow.Identifier,
	) error
}

// ScheduledTransactionsIndex provides full read and write access to the scheduled transactions index.
type ScheduledTransactionsIndex interface {
	ScheduledTransactionsIndexReader
	ScheduledTransactionsIndexRangeReader
	ScheduledTransactionsIndexWriter
}

// ScheduledTransactionsIndexBootstrapper wraps [ScheduledTransactionsIndex] and performs
// just-in-time initialization of the index when the initial block is provided.
//
// All read and write methods proxy to the underlying index once initialized.
type ScheduledTransactionsIndexBootstrapper interface {
	ScheduledTransactionsIndexReader
	ScheduledTransactionsIndexWriter

	// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
	//
	// Expected error returns during normal operation:
	//   - [ErrNotBootstrapped]: if the index has not been initialized
	FirstIndexedHeight() (uint64, error)

	// LatestIndexedHeight returns the latest block height that has been indexed.
	//
	// Expected error returns during normal operation:
	//   - [ErrNotBootstrapped]: if the index has not been initialized
	LatestIndexedHeight() (uint64, error)

	// UninitializedFirstHeight returns the height the index will accept as the first
	// height, and a boolean indicating whether the index is initialized.
	UninitializedFirstHeight() (uint64, bool)
}
