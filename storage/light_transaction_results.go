package storage

import "github.com/onflow/flow-go/model/flow"

// LightTransactionResultsReader represents persistent storage read operations for light transaction result
type LightTransactionResultsReader interface {
	// ByBlockIDTransactionID returns the transaction result for the given block ID and transaction ID
	//
	// Expected errors during normal operation:
	//   - `storage.ErrNotFound` if light transaction result at given blockID wasn't found.
	ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.LightTransactionResult, error)

	// ByBlockIDTransactionIndex returns the transaction result for the given blockID and transaction index
	//
	// Expected errors during normal operation:
	//   - `storage.ErrNotFound` if light transaction result at given blockID and txIndex wasn't found.
	ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.LightTransactionResult, error)

	// ByBlockID gets all transaction results for a block, ordered by transaction index
	//
	// Expected errors during normal operation:
	//   - `storage.ErrNotFound` if light transaction results at given blockID weren't found.
	ByBlockID(id flow.Identifier) ([]flow.LightTransactionResult, error)
}

// LightTransactionResults represents persistent storage for light transaction result
type LightTransactionResults interface {
	LightTransactionResultsReader

	// BatchStore inserts a batch of transaction result into a batch
	BatchStore(blockID flow.Identifier, transactionResults []flow.LightTransactionResult, rw ReaderBatchWriter) error

	// Deprecated: deprecated as a part of transition from Badger to Pebble. use BatchStore instead
	BatchStoreBadger(blockID flow.Identifier, transactionResults []flow.LightTransactionResult, batch BatchStorage) error
}
