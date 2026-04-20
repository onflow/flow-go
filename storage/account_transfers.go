package storage

import (
	"github.com/jordanschalm/lockctx"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// FungibleTokenTransferIterator is an iterator over fungible token transfers ordered by
// descending block height, then ascending transaction and event index within each block.
type FungibleTokenTransferIterator = IndexIterator[accessmodel.FungibleTokenTransfer, accessmodel.TransferCursor]

// FungibleTokenTransfersReader provides read access to the fungible token transfer index.
//
// All methods are safe for concurrent access.
type FungibleTokenTransfersReader interface {
	// ByAddress returns an iterator over fungible token transfers involving the given account,
	// ordered in descending block height (newest first), with ascending transaction and event
	// index within each block. This includes transfers where the account is either the sender
	// or recipient. Returns an exhausted iterator and no error if the account has no transfers.
	//
	// `cursor` is a pointer to an [accessmodel.TransferCursor]:
	//   - nil means start from the latest indexed height
	//   - non-nil means start at the cursor position (inclusive)
	//
	// Expected error returns during normal operations:
	//   - [ErrNotBootstrapped] if the index has not been initialized
	//   - [ErrHeightNotIndexed] if the cursor height extends beyond indexed heights
	ByAddress(
		account flow.Address,
		cursor *accessmodel.TransferCursor,
	) (FungibleTokenTransferIterator, error)
}

// FungibleTokenTransfersRangeReader provides access to the range of available indexed heights.
//
// All methods are safe for concurrent access.
type FungibleTokenTransfersRangeReader interface {
	// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
	FirstIndexedHeight() uint64

	// LatestIndexedHeight returns the latest block height that has been indexed.
	LatestIndexedHeight() uint64
}

// FungibleTokenTransfersWriter provides write access to the fungible token transfer index.
//
// NOT CONCURRENCY SAFE.
type FungibleTokenTransfersWriter interface {
	// Store indexes all fungible token transfers for a block.
	// Each transfer is indexed under both the source and recipient addresses.
	// Must be called sequentially with consecutive heights (latestHeight + 1).
	// The caller must hold the [LockIndexFungibleTokenTransfers] lock until the batch is committed.
	//
	// Expected error returns during normal operations:
	//   - [ErrAlreadyExists] if the block height is already indexed
	Store(lctx lockctx.Proof, rw ReaderBatchWriter, blockHeight uint64, transfers []accessmodel.FungibleTokenTransfer) error
}

// FungibleTokenTransfers provides both read and write access to the fungible token transfer index.
type FungibleTokenTransfers interface {
	FungibleTokenTransfersReader
	FungibleTokenTransfersRangeReader
	FungibleTokenTransfersWriter
}

// FungibleTokenTransfersBootstrapper is a wrapper around the [FungibleTokenTransfers] database that
// performs just-in-time initialization of the index when the initial block is provided.
//
// Fungible token transfers are indexed from execution data which may not be available for the root
// block during bootstrapping. This module acts as a proxy for the underlying [FungibleTokenTransfers]
// and encapsulates the complexity of initializing the index when the initial block is eventually
// provided.
type FungibleTokenTransfersBootstrapper interface {
	FungibleTokenTransfersReader
	FungibleTokenTransfersWriter

	// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
	//
	// Expected error returns during normal operations:
	//   - [ErrNotBootstrapped]: if the index has not been initialized
	FirstIndexedHeight() (uint64, error)

	// LatestIndexedHeight returns the latest block height that has been indexed.
	//
	// Expected error returns during normal operations:
	//   - [ErrNotBootstrapped]: if the index has not been initialized
	LatestIndexedHeight() (uint64, error)

	// UninitializedFirstHeight returns the height the index will accept as the first height, and a boolean
	// indicating if the index is initialized.
	// If the index is not initialized, the first call to `Store` must include data for this height.
	UninitializedFirstHeight() (uint64, bool)
}

// NonFungibleTokenTransferIterator is an iterator over non-fungible token transfers ordered by
// descending block height, then ascending transaction and event index within each block.
type NonFungibleTokenTransferIterator = IndexIterator[accessmodel.NonFungibleTokenTransfer, accessmodel.TransferCursor]

// NonFungibleTokenTransfersReader provides read access to the non-fungible token transfer index.
//
// All methods are safe for concurrent access.
type NonFungibleTokenTransfersReader interface {
	// ByAddress returns an iterator over non-fungible token transfers involving the given account,
	// ordered in descending block height (newest first), with ascending transaction and event
	// index within each block. This includes transfers where the account is either the sender
	// or recipient. Returns an exhausted iterator and no error if the account has no transfers.
	//
	// `cursor` is a pointer to an [accessmodel.TransferCursor]:
	//   - nil means start from the latest indexed height
	//   - non-nil means start at the cursor position (inclusive)
	//
	// Expected error returns during normal operations:
	//   - [ErrNotBootstrapped] if the index has not been initialized
	//   - [ErrHeightNotIndexed] if the cursor height extends beyond indexed heights
	ByAddress(
		account flow.Address,
		cursor *accessmodel.TransferCursor,
	) (NonFungibleTokenTransferIterator, error)
}

// NonFungibleTokenTransfersRangeReader provides access to the range of available indexed heights.
//
// All methods are safe for concurrent access.
type NonFungibleTokenTransfersRangeReader interface {
	// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
	FirstIndexedHeight() uint64

	// LatestIndexedHeight returns the latest block height that has been indexed.
	LatestIndexedHeight() uint64
}

// NonFungibleTokenTransfersWriter provides write access to the non-fungible token transfer index.
//
// NOT CONCURRENCY SAFE.
type NonFungibleTokenTransfersWriter interface {
	// Store indexes all non-fungible token transfers for a block.
	// Each transfer is indexed under both the source and recipient addresses.
	// Must be called sequentially with consecutive heights (latestHeight + 1).
	// The caller must hold the [LockIndexNonFungibleTokenTransfers] lock until the batch is committed.
	//
	// Expected error returns during normal operations:
	//   - [ErrAlreadyExists] if the block height is already indexed
	Store(lctx lockctx.Proof, rw ReaderBatchWriter, blockHeight uint64, transfers []accessmodel.NonFungibleTokenTransfer) error
}

// NonFungibleTokenTransfers provides both read and write access to the non-fungible token transfer index.
type NonFungibleTokenTransfers interface {
	NonFungibleTokenTransfersReader
	NonFungibleTokenTransfersRangeReader
	NonFungibleTokenTransfersWriter
}

// NonFungibleTokenTransfersBootstrapper is a wrapper around the [NonFungibleTokenTransfers] database that
// performs just-in-time initialization of the index when the initial block is provided.
//
// Non-fungible token transfers are indexed from execution data which may not be available for the root
// block during bootstrapping. This module acts as a proxy for the underlying [NonFungibleTokenTransfers]
// and encapsulates the complexity of initializing the index when the initial block is eventually
// provided.
type NonFungibleTokenTransfersBootstrapper interface {
	NonFungibleTokenTransfersReader
	NonFungibleTokenTransfersWriter

	// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
	//
	// Expected error returns during normal operations:
	//   - [ErrNotBootstrapped]: if the index has not been initialized
	FirstIndexedHeight() (uint64, error)

	// LatestIndexedHeight returns the latest block height that has been indexed.
	//
	// Expected error returns during normal operations:
	//   - [ErrNotBootstrapped]: if the index has not been initialized
	LatestIndexedHeight() (uint64, error)

	// UninitializedFirstHeight returns the height the index will accept as the first height, and a boolean
	// indicating if the index is initialized.
	// If the index is not initialized, the first call to `Store` must include data for this height.
	UninitializedFirstHeight() (uint64, bool)
}
