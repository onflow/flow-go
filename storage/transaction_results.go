package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

type TransactionResultsReader interface {
	// ByBlockIDTransactionID returns the transaction result for the given block ID and transaction ID
	ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.TransactionResult, error)

	// ByBlockIDTransactionIndex returns the transaction result for the given blockID and transaction index
	ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.TransactionResult, error)

	// ByBlockID gets all transaction results for a block, ordered by transaction index
	ByBlockID(id flow.Identifier) ([]flow.TransactionResult, error)
}

// TransactionResults represents persistent storage for transaction result
type TransactionResults interface {
	TransactionResultsReader

	// BatchStore inserts a batch of transaction result into a batch
	// It returns [ErrAlreadyExists] if transaction results for the block already exist.
	// It requires the caller to hold [storage.LockInsertAndIndexTxResult]
	BatchStore(lctx lockctx.Proof, rw ReaderBatchWriter, blockID flow.Identifier, transactionResults []flow.TransactionResult) error

	// RemoveByBlockID removes all transaction results for a block
	BatchRemoveByBlockID(id flow.Identifier, batch ReaderBatchWriter) error
}
