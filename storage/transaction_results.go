package storage

import "github.com/onflow/flow-go/model/flow"

// TransactionResults represents persistent storage for transaction result
type TransactionResults interface {

	// BatchStore inserts a batch of transaction result into a batch
	BatchStore(blockID flow.Identifier, transactionResults []flow.TransactionResult, batch BatchStorage) error

	// ByBlockIDTransactionID returns the transaction result for the given block ID and transaction ID
	ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.TransactionResult, error)

	// ByBlockIDTransactionIndex returns the transaction result for the given blockID and transaction index
	ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.TransactionResult, error)

	// ByBlockID gets all transaction results for a block, ordered by transaction index
	ByBlockID(id flow.Identifier) ([]flow.TransactionResult, error)
}

// LightTransactionResults represents persistent storage for light transaction result
type LightTransactionResults interface {

	// BatchStore inserts a batch of transaction result into a batch
	BatchStore(blockID flow.Identifier, transactionResults []flow.LightTransactionResult, batch BatchStorage) error

	// ByBlockIDTransactionID returns the transaction result for the given block ID and transaction ID
	ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.LightTransactionResult, error)

	// ByBlockIDTransactionIndex returns the transaction result for the given blockID and transaction index
	ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.LightTransactionResult, error)

	// ByBlockID gets all transaction results for a block, ordered by transaction index
	ByBlockID(id flow.Identifier) ([]flow.LightTransactionResult, error)
}

// TransactionResultErrorMessages represents persistent storage for transaction result error messages
type TransactionResultErrorMessages interface {

	// Store will store transaction result error messages for the given block ID.
	//
	// No errors are expected during normal operation.
	Store(blockID flow.Identifier, transactionResultErrorMessages []flow.TransactionResultErrorMessage) error

	// Exists returns true if transaction result error messages for the given ID have been stored.
	//
	// No errors are expected during normal operation.
	Exists(blockID flow.Identifier) (bool, error)

	// ByBlockIDTransactionID returns the transaction result error message for the given block ID and transaction ID.
	//
	// Expected errors during normal operation:
	//   - `storage.ErrNotFound` if no transaction error message is known at given block and transaction id.
	ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.TransactionResultErrorMessage, error)

	// ByBlockIDTransactionIndex returns the transaction result error message for the given blockID and transaction index.
	//
	// Expected errors during normal operation:
	//   - `storage.ErrNotFound` if no transaction error message is known at given block and transaction index.
	ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.TransactionResultErrorMessage, error)

	// ByBlockID gets all transaction result error messages for a block, ordered by transaction index.
	// Note: This method will return an empty slice both if the block is not indexed yet and if the block does not have any errors.
	//
	// No errors are expected during normal operation.
	ByBlockID(id flow.Identifier) ([]flow.TransactionResultErrorMessage, error)
}
