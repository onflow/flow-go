package storage

import (
	"github.com/jordanschalm/lockctx"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// AccountTransactionsReader provides read access to the account transaction index.
//
// All methods are safe for concurrent access.
type AccountTransactionsReader interface {
	// TransactionsByAddress retrieves transaction references for an account within the specified
	// inclusive block height range.
	// Results are returned in descending order (newest first).
	//
	// startHeight and endHeight are inclusive. If endHeight is greater than the latest indexed height,
	// the latest indexed height will be used.
	//
	// Expected error returns during normal operations:
	//   - [ErrHeightNotIndexed] if the requested range extends beyond indexed heights
	//   - [ErrInvalidQuery] if the query parameters are invalid (e.g., startHeight > endHeight)
	TransactionsByAddress(
		account flow.Address,
		startHeight uint64,
		endHeight uint64,
	) ([]accessmodel.AccountTransaction, error)
}

// AccountTransactionsRangeReader provides access to the range of available indexed heights.
//
// All methods are safe for concurrent access.
type AccountTransactionsRangeReader interface {
	// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
	FirstIndexedHeight() uint64

	// LatestIndexedHeight returns the latest block height that has been indexed.
	LatestIndexedHeight() uint64
}

// AccountTransactionsWriter provides write access to the account transaction index.
//
// NOT CONCURRENTLY SAFE.
type AccountTransactionsWriter interface {
	// Store indexes all account-transaction associations for a block.
	// Must be called sequentially with consecutive heights (latestHeight + 1).
	// The caller must hold the [storage.LockIndexAccountTransactions] lock until the batch is committed.
	//
	// Expected error returns during normal operations:
	//   - [storage.ErrAlreadyExists] if the block height is already indexed
	Store(lctx lockctx.Proof, rw ReaderBatchWriter, blockHeight uint64, txData []accessmodel.AccountTransaction) error
}

// AccountTransactions provides both read and write access to the account transaction index.
type AccountTransactions interface {
	AccountTransactionsReader
	AccountTransactionsRangeReader
	AccountTransactionsWriter
}

// AccountTransactionsBootstrapper is a wrapper around the [AccountTransactions] database that performs
// just-in-time initialization of the index when the initial block is provided.
//
// Account transactions are indexed from execution data which may not be available for the root block
// during bootstrapping. This module acts as a proxy for the underlying [AccountTransactions] and
// encapsulates the complexity of initializing the index when the initial block is eventually provided.
type AccountTransactionsBootstrapper interface {
	AccountTransactionsReader
	AccountTransactionsWriter

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
