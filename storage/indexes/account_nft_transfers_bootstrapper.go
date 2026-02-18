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

// NonFungibleTokenTransfersBootstrapper wraps a [NonFungibleTokenTransfers] and performs just-in-time
// initialization of the index when the initial block is provided.
//
// Non-fungible token transfers are indexed from execution data which may not be available for the root
// block during bootstrapping. This module acts as a proxy for the underlying [NonFungibleTokenTransfers]
// and encapsulates the complexity of initializing the index when the initial block is eventually
// provided.
type NonFungibleTokenTransfersBootstrapper struct {
	db                 storage.DB
	initialStartHeight uint64

	store *atomic.Pointer[NonFungibleTokenTransfers]
}

var _ storage.NonFungibleTokenTransfersBootstrapper = (*NonFungibleTokenTransfersBootstrapper)(nil)

// NewNonFungibleTokenTransfersBootstrapper creates a new [NonFungibleTokenTransfersBootstrapper].
// If the index is already initialized (from a previous run), the underlying store is loaded immediately.
// Otherwise, the store remains nil until the first call to [Store] with the initial start height.
//
// No error returns are expected during normal operation.
func NewNonFungibleTokenTransfersBootstrapper(db storage.DB, initialStartHeight uint64) (*NonFungibleTokenTransfersBootstrapper, error) {
	store, err := NewNonFungibleTokenTransfers(db)
	if err != nil {
		if !errors.Is(err, storage.ErrNotBootstrapped) {
			return nil, fmt.Errorf("could not create non-fungible token transfers: %w", err)
		}
		store = nil
	}

	return &NonFungibleTokenTransfersBootstrapper{
		db:                 db,
		initialStartHeight: initialStartHeight,
		store:              atomic.NewPointer(store),
	}, nil
}

// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
func (b *NonFungibleTokenTransfersBootstrapper) FirstIndexedHeight() (uint64, error) {
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
func (b *NonFungibleTokenTransfersBootstrapper) LatestIndexedHeight() (uint64, error) {
	store := b.store.Load()
	if store == nil {
		return 0, storage.ErrNotBootstrapped
	}
	return store.LatestIndexedHeight(), nil
}

// UninitializedFirstHeight returns the height the index will accept as the first height, and a boolean
// indicating if the index is initialized.
// If the index is not initialized, the first call to `Store` must include data for this height.
func (b *NonFungibleTokenTransfersBootstrapper) UninitializedFirstHeight() (uint64, bool) {
	store := b.store.Load()
	if store == nil {
		return b.initialStartHeight, false
	}
	return store.FirstIndexedHeight(), true
}

// TransfersByAddress retrieves non-fungible token transfers involving the given account using
// cursor-based pagination. Results are returned in descending order (newest first).
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
//   - [storage.ErrHeightNotIndexed] if the cursor height extends beyond indexed heights
func (b *NonFungibleTokenTransfersBootstrapper) TransfersByAddress(
	account flow.Address,
	limit uint32,
	cursor *access.TransferCursor,
	filter storage.IndexFilter[*access.NonFungibleTokenTransfer],
) (access.NonFungibleTokenTransfersPage, error) {
	store := b.store.Load()
	if store == nil {
		return access.NonFungibleTokenTransfersPage{}, storage.ErrNotBootstrapped
	}
	return store.TransfersByAddress(account, limit, cursor, filter)
}

// Store indexes all non-fungible token transfers for a block.
// Must be called sequentially with consecutive heights (latestHeight + 1).
// Calling with the last height is a no-op.
// The caller must hold the [storage.LockIndexNonFungibleTokenTransfers] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized and the provided block height
//     is not the initial start height
//   - [storage.ErrAlreadyExists] if the block height is already indexed
func (b *NonFungibleTokenTransfersBootstrapper) Store(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64, transfers []access.NonFungibleTokenTransfer) error {
	// if the index is already initialized, store the data directly
	if store := b.store.Load(); store != nil {
		return store.Store(lctx, rw, blockHeight, transfers)
	}

	// otherwise bootstrap the index
	if blockHeight != b.initialStartHeight {
		return fmt.Errorf("expected first indexed height %d, got %d: %w", b.initialStartHeight, blockHeight, storage.ErrNotBootstrapped)
	}

	store, err := BootstrapNonFungibleTokenTransfers(lctx, rw, b.db, b.initialStartHeight, transfers)
	if err != nil {
		return fmt.Errorf("could not initialize non-fungible token transfers storage: %w", err)
	}

	if !b.store.CompareAndSwap(nil, store) {
		return fmt.Errorf("non-fungible token transfers initialized during bootstrap")
	}

	return nil
}
