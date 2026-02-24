package indexes

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// ScheduledTransactionsBootstrapper wraps a [ScheduledTransactionsIndex] and performs
// just-in-time initialization of the index when the initial block is provided.
//
// Scheduled transactions may not be available for the root block during bootstrapping.
// This struct acts as a proxy for the underlying [ScheduledTransactionsIndex] and
// encapsulates the complexity of initializing the index when the initial block is eventually provided.
type ScheduledTransactionsBootstrapper struct {
	db                 storage.DB
	initialStartHeight uint64

	store *atomic.Pointer[ScheduledTransactionsIndex]
}

var _ storage.ScheduledTransactionsIndexBootstrapper = (*ScheduledTransactionsBootstrapper)(nil)

// NewScheduledTransactionsBootstrapper creates a new scheduled transactions bootstrapper.
//
// No error returns are expected during normal operation.
func NewScheduledTransactionsBootstrapper(db storage.DB, initialStartHeight uint64) (*ScheduledTransactionsBootstrapper, error) {
	store, err := NewScheduledTransactionsIndex(db)
	if err != nil {
		if !errors.Is(err, storage.ErrNotBootstrapped) {
			return nil, fmt.Errorf("could not create scheduled transactions index: %w", err)
		}
		// make sure it's nil
		store = nil
	}

	return &ScheduledTransactionsBootstrapper{
		db:                 db,
		initialStartHeight: initialStartHeight,
		store:              atomic.NewPointer(store),
	}, nil
}

// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
func (b *ScheduledTransactionsBootstrapper) FirstIndexedHeight() (uint64, error) {
	store := b.store.Load()
	if store == nil {
		return 0, storage.ErrNotBootstrapped
	}
	return store.FirstIndexedHeight(), nil
}

// LatestIndexedHeight returns the latest block height that has been indexed.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
func (b *ScheduledTransactionsBootstrapper) LatestIndexedHeight() (uint64, error) {
	store := b.store.Load()
	if store == nil {
		return 0, storage.ErrNotBootstrapped
	}
	return store.LatestIndexedHeight(), nil
}

// UninitializedFirstHeight returns the height the index will accept as the first height, and a boolean
// indicating if the index is initialized.
// If the index is not initialized, the first call to `Store` must include data for this height.
func (b *ScheduledTransactionsBootstrapper) UninitializedFirstHeight() (uint64, bool) {
	store := b.store.Load()
	if store == nil {
		return b.initialStartHeight, false
	}
	return store.FirstIndexedHeight(), true
}

// ByID returns the scheduled transaction with the given scheduler-assigned ID.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
//   - [storage.ErrNotFound] if no scheduled transaction with the given ID exists
func (b *ScheduledTransactionsBootstrapper) ByID(id uint64) (access.ScheduledTransaction, error) {
	store := b.store.Load()
	if store == nil {
		return access.ScheduledTransaction{}, storage.ErrNotBootstrapped
	}
	return store.ByID(id)
}

// ByAddress retrieves scheduled transactions for an account using cursor-based pagination.
// Results are returned in descending order (highest ID first).
//
// `limit` specifies the maximum number of results per page.
//
// `cursor` is a pointer to a [access.ScheduledTransactionCursor]:
//   - nil means start from the highest indexed ID (first page)
//   - non-nil means resume after the cursor position (subsequent pages)
//
// `filter` is an optional filter applied before calculating the limit. If nil, all
// transactions are returned. The same filter must be applied across all pages.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
//   - [storage.ErrInvalidQuery] if limit is invalid
func (b *ScheduledTransactionsBootstrapper) ByAddress(
	account flow.Address,
	limit uint32,
	cursor *access.ScheduledTransactionCursor,
	filter storage.IndexFilter[*access.ScheduledTransaction],
) (access.ScheduledTransactionsPage, error) {
	store := b.store.Load()
	if store == nil {
		return access.ScheduledTransactionsPage{}, storage.ErrNotBootstrapped
	}
	return store.ByAddress(account, limit, cursor, filter)
}

// All retrieves all scheduled transactions using cursor-based pagination.
// Results are returned in descending order (highest ID first).
//
// `limit` specifies the maximum number of results per page.
//
// `cursor` is a pointer to a [access.ScheduledTransactionCursor]:
//   - nil means start from the highest indexed ID (first page)
//   - non-nil means resume after the cursor position (subsequent pages)
//
// `filter` is an optional filter applied before calculating the limit. If nil, all
// transactions are returned. The same filter must be applied across all pages.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
//   - [storage.ErrInvalidQuery] if limit is invalid
func (b *ScheduledTransactionsBootstrapper) All(
	limit uint32,
	cursor *access.ScheduledTransactionCursor,
	filter storage.IndexFilter[*access.ScheduledTransaction],
) (access.ScheduledTransactionsPage, error) {
	store := b.store.Load()
	if store == nil {
		return access.ScheduledTransactionsPage{}, storage.ErrNotBootstrapped
	}
	return store.All(limit, cursor, filter)
}

// Store indexes all new scheduled transactions from the given block.
// Must be called sequentially with consecutive heights (latestHeight + 1).
// The caller must hold the [storage.LockIndexScheduledTransactionsIndex] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized and the provided block height
//     is not the initial start height
//   - [storage.ErrAlreadyExists] if the block height is already indexed
func (b *ScheduledTransactionsBootstrapper) Store(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	blockHeight uint64,
	scheduledTxs []access.ScheduledTransaction,
) error {
	// if the index is already initialized, store the data directly
	if store := b.store.Load(); store != nil {
		return store.Store(lctx, rw, blockHeight, scheduledTxs)
	}

	// otherwise bootstrap the index. this will store the data during initialization
	if blockHeight != b.initialStartHeight {
		return fmt.Errorf("expected first indexed height %d, got %d: %w", b.initialStartHeight, blockHeight, storage.ErrNotBootstrapped)
	}

	store, err := BootstrapScheduledTransactions(lctx, rw, b.db, b.initialStartHeight, scheduledTxs)
	if err != nil {
		return fmt.Errorf("could not initialize scheduled transactions storage: %w", err)
	}

	if !b.store.CompareAndSwap(nil, store) {
		// this should never happen. if it does, there is a bug. this indicates another goroutine
		// successfully initialized `store` since we checked the value above. since the bootstrap
		// operation is protected by the lock and it performs sanity checks to ensure the table
		// is actually empty, the bootstrap operation should fail if there was concurrent access.
		return fmt.Errorf("scheduled transactions initialized during bootstrap")
	}

	return nil
}

// Executed updates the scheduled transaction's status to Executed and records the ID of the transaction
// that emitted the Executed event.
// The caller must hold the [storage.LockIndexScheduledTransactionsIndex] lock until the batch
// is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
//   - [storage.ErrNotFound] if no scheduled transaction with the given ID exists
//   - [storage.ErrInvalidStatusTransition] if the transaction is already Executed, Cancelled, or Failed
func (b *ScheduledTransactionsBootstrapper) Executed(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	scheduledTxID uint64,
	transactionID flow.Identifier,
) error {
	store := b.store.Load()
	if store == nil {
		return storage.ErrNotBootstrapped
	}
	return store.Executed(lctx, rw, scheduledTxID, transactionID)
}

// Cancelled updates the scheduled transaction's status to Cancelled and records the
// fee amounts and the ID of the transaction that emitted the Canceled event.
// The caller must hold the [storage.LockIndexScheduledTransactionsIndex] lock until the batch
// is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
//   - [storage.ErrNotFound] if no scheduled transaction with the given ID exists
//   - [storage.ErrInvalidStatusTransition] if the transaction is already Executed, Cancelled, or Failed
func (b *ScheduledTransactionsBootstrapper) Cancelled(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	scheduledTxID uint64,
	feesReturned uint64,
	feesDeducted uint64,
	transactionID flow.Identifier,
) error {
	store := b.store.Load()
	if store == nil {
		return storage.ErrNotBootstrapped
	}
	return store.Cancelled(lctx, rw, scheduledTxID, feesReturned, feesDeducted, transactionID)
}

// Failed updates the transaction's status to Failed and records the ID of the transaction
// that emitted the Failed event.
// The caller must hold the [storage.LockIndexScheduledTransactionsIndex] lock until committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
//   - [storage.ErrNotFound] if no scheduled transaction with the given ID exists
//   - [storage.ErrInvalidStatusTransition] if the transaction is already Executed, Cancelled, or Failed
func (b *ScheduledTransactionsBootstrapper) Failed(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	scheduledTxID uint64,
	transactionID flow.Identifier,
) error {
	store := b.store.Load()
	if store == nil {
		return storage.ErrNotBootstrapped
	}
	return store.Failed(lctx, rw, scheduledTxID, transactionID)
}
