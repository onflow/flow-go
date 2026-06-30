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

// FungibleTokenTransfersBootstrapper wraps a [FungibleTokenTransfers] and performs just-in-time
// initialization of the index when the initial block is provided.
//
// Fungible token transfers are indexed from execution data which may not be available for the root
// block during bootstrapping. This module acts as a proxy for the underlying [FungibleTokenTransfers]
// and encapsulates the complexity of initializing the index when the initial block is eventually
// provided.
type FungibleTokenTransfersBootstrapper struct {
	db                 storage.DB
	initialStartHeight uint64

	store *atomic.Pointer[FungibleTokenTransfers]
}

var _ storage.FungibleTokenTransfersBootstrapper = (*FungibleTokenTransfersBootstrapper)(nil)

// NewFungibleTokenTransfersBootstrapper creates a new [FungibleTokenTransfersBootstrapper].
// If the index is already initialized (from a previous run), the underlying store is loaded immediately.
// Otherwise, the store remains nil until the first call to [Store] with the initial start height.
//
// No error returns are expected during normal operation.
func NewFungibleTokenTransfersBootstrapper(db storage.DB, initialStartHeight uint64) (*FungibleTokenTransfersBootstrapper, error) {
	store, err := NewFungibleTokenTransfers(db)
	if err != nil {
		if !errors.Is(err, storage.ErrNotBootstrapped) {
			return nil, fmt.Errorf("could not create fungible token transfers: %w", err)
		}
		store = nil
	}

	return &FungibleTokenTransfersBootstrapper{
		db:                 db,
		initialStartHeight: initialStartHeight,
		store:              atomic.NewPointer(store),
	}, nil
}

// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
func (b *FungibleTokenTransfersBootstrapper) FirstIndexedHeight() (uint64, error) {
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
func (b *FungibleTokenTransfersBootstrapper) LatestIndexedHeight() (uint64, error) {
	store := b.store.Load()
	if store == nil {
		return 0, storage.ErrNotBootstrapped
	}
	return store.LatestIndexedHeight(), nil
}

// UninitializedFirstHeight returns the height the index will accept as the first height, and a boolean
// indicating if the index is initialized.
// If the index is not initialized, the first call to `Store` must include data for this height.
func (b *FungibleTokenTransfersBootstrapper) UninitializedFirstHeight() (uint64, bool) {
	store := b.store.Load()
	if store == nil {
		return b.initialStartHeight, false
	}
	return store.FirstIndexedHeight(), true
}

// ByAddress returns an iterator over fungible token transfers involving the given account.
// See [FungibleTokenTransfers.ByAddress] for full documentation.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized
//   - [storage.ErrHeightNotIndexed] if the cursor height extends beyond indexed heights
func (b *FungibleTokenTransfersBootstrapper) ByAddress(
	account flow.Address,
	cursor *access.TransferCursor,
) (storage.FungibleTokenTransferIterator, error) {
	store := b.store.Load()
	if store == nil {
		return nil, storage.ErrNotBootstrapped
	}
	return store.ByAddress(account, cursor)
}

// Store indexes all fungible token transfers for a block.
// Must be called sequentially with consecutive heights (latestHeight + 1).
// The caller must hold the [storage.LockIndexFungibleTokenTransfers] lock until the batch is committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotBootstrapped] if the index has not been initialized and the provided block height
//     is not the initial start height
//   - [storage.ErrAlreadyExists] if the block height is already indexed
func (b *FungibleTokenTransfersBootstrapper) Store(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64, transfers []access.FungibleTokenTransfer) error {
	// if the index is already initialized, store the data directly
	if store := b.store.Load(); store != nil {
		return store.Store(lctx, rw, blockHeight, transfers)
	}

	// otherwise bootstrap the index
	if blockHeight != b.initialStartHeight {
		return fmt.Errorf("expected first indexed height %d, got %d: %w", b.initialStartHeight, blockHeight, storage.ErrNotBootstrapped)
	}

	store, err := BootstrapFungibleTokenTransfers(lctx, rw, b.db, b.initialStartHeight, transfers)
	if err != nil {
		return fmt.Errorf("could not initialize fungible token transfers storage: %w", err)
	}

	if !b.store.CompareAndSwap(nil, store) {
		return fmt.Errorf("fungible token transfers initialized during bootstrap")
	}

	return nil
}
