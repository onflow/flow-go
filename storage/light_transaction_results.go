package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

// LightTransactionResultsReader represents persistent storage read operations for light transaction result
type LightTransactionResultsReader interface {
	// ByBlockIDTransactionID returns the transaction result for the given block ID and transaction ID
	//
	// Expected error returns during normal operation:
	//   - [storage.ErrNotFound] if light transaction result at given blockID wasn't found.
	ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.LightTransactionResult, error)

	// ByBlockIDTransactionIndex returns the transaction result for the given blockID and transaction index
	//
	// Expected error returns during normal operation:
	//   - [storage.ErrNotFound] if light transaction result at given blockID and txIndex wasn't found.
	ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.LightTransactionResult, error)

	// ByBlockID gets all transaction results for a block, ordered by transaction index
	// CAUTION: this function returns the empty list in case for block IDs without known results.
	//
	// No error returns are expected during normal operations.
	ByBlockID(id flow.Identifier) ([]flow.LightTransactionResult, error)
}

// LightTransactionResults represents persistent storage for light transaction result
type LightTransactionResults interface {
	LightTransactionResultsReader

	// BatchStore persists and indexes all transaction results (light representation) for the given blockID
	// as part of the provided batch. The caller must acquire [storage.LockInsertLightTransactionResult] and
	// hold it until the write batch has been committed.
	//
	// Expected error returns during normal operation:
	//   - [storage.ErrAlreadyExists] if light transaction results for the block already exist.
	BatchStore(lctx lockctx.Proof, rw ReaderBatchWriter, blockID flow.Identifier, transactionResults []flow.LightTransactionResult) error
}
