package storage

import (
	"github.com/jordanschalm/lockctx"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// IndexFilter is a function that filters data entries to include in query responses.
// It takes a single entry and returns true if the entry should be included in the response.
type IndexFilter[T any] func(T) bool

// AccountTransactionsReader provides read access to the account transaction index.
//
// All methods are safe for concurrent access.
type AccountTransactionsReader interface {
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
	TransactionsByAddress(
		account flow.Address,
		limit uint32,
		cursor *accessmodel.AccountTransactionCursor,
		filter IndexFilter[*accessmodel.AccountTransaction],
	) (accessmodel.AccountTransactionsPage, error)
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
