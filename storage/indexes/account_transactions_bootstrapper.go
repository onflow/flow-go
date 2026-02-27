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

// AccountTransactionsBootstrapper wraps an [AccountTransactions] and performs just-in-time initialization
// of the index when the initial block is provided.
//
// Account transactions are indexed from execution data which may not be available for the root block
// during bootstrapping. This module acts as a proxy for the underlying [AccountTransactions] and
// encapsulates the complexity of initializing the index when the initial block is eventually provided.
type AccountTransactionsBootstrapper struct {
	db                 storage.DB
	initialStartHeight uint64

	store *atomic.Pointer[AccountTransactions]
}

var _ storage.AccountTransactionsBootstrapper = (*AccountTransactionsBootstrapper)(nil)

// NewAccountTransactionsBootstrapper creates a new account transactions bootstrapper.
//
// No error returns are expected during normal operation.
func NewAccountTransactionsBootstrapper(db storage.DB, initialStartHeight uint64) (*AccountTransactionsBootstrapper, error) {
	store, err := NewAccountTransactions(db)
	if err != nil {
		if !errors.Is(err, storage.ErrNotBootstrapped) {
			return nil, fmt.Errorf("could not create account transactions: %w", err)
		}
		// make sure it's nil
		store = nil
	}

	return &AccountTransactionsBootstrapper{
		db:                 db,
		initialStartHeight: initialStartHeight,
		store:              atomic.NewPointer[AccountTransactions](store),
	}, nil
}

// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
func (b *AccountTransactionsBootstrapper) FirstIndexedHeight() (uint64, error) {
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
func (b *AccountTransactionsBootstrapper) LatestIndexedHeight() (uint64, error) {
	store := b.store.Load()
	if store == nil {
		return 0, storage.ErrNotBootstrapped
	}
	return store.LatestIndexedHeight(), nil
}

// UninitializedFirstHeight returns the height the index will accept as the first height, and a boolean
// indicating if the index is initialized.
// If the index is not initialized, the first call to `Store` must include data for this height.
func (b *AccountTransactionsBootstrapper) UninitializedFirstHeight() (uint64, bool) {
	store := b.store.Load()
	if store == nil {
		return b.initialStartHeight, false
	}
	return store.FirstIndexedHeight(), true
}

// TransactionsByAddress retrieves transaction references for an account using cursor-based pagination.
// Results are returned in descending order (newest first).
//
// `limit` specifies the maximum number of results to return per page.
//
// `cursor` is a pointer to an [access.AccountTransactionCursor]:
//   - nil means start from the latest indexed height (first page)
//   - non-nil means resume after the cursor position (subsequent pages)
//
// `filter` is an optional filter to apply to the results. If nil, all transactions will be returned.
// The filter is applied before calculating the limit. For pagination, to work correctly, the same
// filter must be applied to all pages.
//
// Expected error returns during normal operations:
//   - [ErrNotBootstrapped] if the index has not been initialized
//   - [storage.ErrHeightNotIndexed] if the cursor height extends beyond indexed heights
//   - [storage.ErrInvalidQuery] if the limit is invalid
func (b *AccountTransactionsBootstrapper) TransactionsByAddress(
	account flow.Address,
	limit uint32,
	cursor *access.AccountTransactionCursor,
	filter storage.IndexFilter[*access.AccountTransaction],
) (access.AccountTransactionsPage, error) {
	store := b.store.Load()
	if store == nil {
		return access.AccountTransactionsPage{}, storage.ErrNotBootstrapped
	}
	return store.TransactionsByAddress(account, limit, cursor, filter)
}

// Store indexes all account-transaction associations for a block.
// Must be called sequentially with consecutive heights (latestHeight + 1).
// Calling with the last height is a no-op.
// The caller must hold the [storage.LockIndexAccountTransactions] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized and the provided block height is not the initial start height
//   - [storage.ErrAlreadyExists] if the block height is already indexed
func (b *AccountTransactionsBootstrapper) Store(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64, txData []access.AccountTransaction) error {
	// if the index is already initialized, store the data directly
	if store := b.store.Load(); store != nil {
		return store.Store(lctx, rw, blockHeight, txData)
	}

	// otherwise bootstrap the index. this will store the data during initialization
	if blockHeight != b.initialStartHeight {
		return fmt.Errorf("expected first indexed height %d, got %d: %w", b.initialStartHeight, blockHeight, storage.ErrNotBootstrapped)
	}

	store, err := BootstrapAccountTransactions(lctx, rw, b.db, b.initialStartHeight, txData)
	if err != nil {
		return fmt.Errorf("could not initialize account transactions storage: %w", err)
	}

	if !b.store.CompareAndSwap(nil, store) {
		// this should never happen. if it does, there is a bug. this indicates another goroutine
		// successfully initialized `store` since we checked the value above. since the bootstrap
		// operation is protected by the lock and it performs sanity checks to ensure the table
		// is actually empty, the bootstrap operation should fail if there was concurrent access.
		return fmt.Errorf("account transactions initialized during bootstrap")
	}

	return nil
}
