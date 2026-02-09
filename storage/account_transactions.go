package storage

import (
	"github.com/jordanschalm/lockctx"
	accessmodel "github.com/onflow/flow-go/model/access"
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
	// Expected error returns during normal operations:
	//   - ErrHeightNotIndexed if the requested range extends beyond indexed heights
	TransactionsByAddress(
		account flow.Address,
		startHeight uint64,
		endHeight uint64,
	) ([]accessmodel.AccountTransaction, error)

	// LatestIndexedHeight returns the latest block height that has been indexed.
	//
	// Expected error returns during normal operations:
	//   - [ErrNotFound]: if the index has not been initialized
	LatestIndexedHeight() (uint64, error)

	// FirstIndexedHeight returns the first (oldest) block height that has been indexed.
	//
	// Expected error returns during normal operations:
	//   - [ErrNotFound]: if the index has not been initialized
	FirstIndexedHeight() (uint64, error)
}

// AccountTransactions provides both read and write access to the account
// transaction index. It combines AccountTransactionsReader and
// AccountTransactionsWriter interfaces.
type AccountTransactions interface {
	AccountTransactionsReader

	// Store indexes all account-transaction associations for a block.
	// This should be called once per block, with consecutive heights.
	//
	// CAUTION: Must be called sequentially with consecutive heights (latestHeight + 1).
	// The caller is responsible for committing the provided batch.
	//
	// No errors are expected during normal operation.
	Store(lctx lockctx.Proof, rw ReaderBatchWriter, blockHeight uint64, txData []accessmodel.AccountTransaction) error
}
