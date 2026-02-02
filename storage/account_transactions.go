package storage

import (
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// AccountTransactionsReader provides read access to the account transaction index.
// This interface allows querying transactions associated with a specific account
// within a block height range. Results are returned in descending order (newest first).
//
// All methods are safe for concurrent access.
type AccountTransactionsReader interface {
	// TransactionsByAddress retrieves transaction references for an account
	// within the specified inclusive block height range.
	// Results are returned in descending order (newest first).
	//
	// startHeight and endHeight are inclusive. If endHeight is greater than the latest indexed height,
	// the latest indexed height will be used.
	//
	// Expected errors during normal operations:
	//   - ErrHeightNotIndexed if the requested range extends beyond indexed heights
	TransactionsByAddress(
		account flow.Address,
		startHeight uint64,
		endHeight uint64,
	) ([]access.AccountTransaction, error)

	// LatestIndexedHeight returns the latest block height that has been indexed.
	//
	// Expected errors during normal operations:
	//   - ErrNotBootstrapped if the index has not been initialized
	LatestIndexedHeight() (uint64, error)

	// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
	//
	// Expected errors during normal operations:
	//   - ErrNotBootstrapped if the index has not been initialized
	FirstIndexedHeight() (uint64, error)
}

// AccountTransactionsWriter provides write access to the account transaction index.
//
// CAUTION: Write operations are not safe for concurrent use. Callers must ensure
// that Store is called sequentially with consecutive heights.
type AccountTransactionsWriter interface {
	// Store indexes all account-transaction associations for a block.
	// This should be called once per block, with consecutive heights.
	//
	// CAUTION: Must be called sequentially with consecutive heights (latestHeight + 1).
	//
	// No errors are expected during normal operation.
	Store(blockHeight uint64, txData []access.AccountTransaction) error
}

// AccountTransactions provides both read and write access to the account
// transaction index. It combines AccountTransactionsReader and
// AccountTransactionsWriter interfaces.
type AccountTransactions interface {
	AccountTransactionsReader
	AccountTransactionsWriter
}
