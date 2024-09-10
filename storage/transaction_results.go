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

	// BatchStore inserts a batch of transaction result error messages into a batch
	BatchStore(blockID flow.Identifier, transactionResultErrorMessages []flow.TransactionResultErrorMessage, batch BatchStorage) error

	// ByBlockIDTransactionID returns the transaction result error message for the given block ID and transaction ID
	ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.TransactionResultErrorMessage, error)

	// ByBlockIDTransactionIndex returns the transaction result error message for the given blockID and transaction index
	ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.TransactionResultErrorMessage, error)

	// ByBlockID gets all transaction result error messages for a block, ordered by transaction index
	ByBlockID(id flow.Identifier) ([]flow.TransactionResultErrorMessage, error)
}
