package storage

import "github.com/onflow/flow-go/model/flow"

// TransactionResults represents persistent storage for transaction result
type TransactionResults interface {

	// Store inserts the transaction result
	Store(blockID flow.Identifier, transactionResult *flow.TransactionResult) error

	// BatchStore inserts a batch of transaction result
	BatchStore(blockID flow.Identifier, transactionResults []flow.TransactionResult) error

	// ByBlockIDTransactionID returns the transaction result for the given block ID and transaction ID
	ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.TransactionResult, error)

	// RemoveByBlockID removes transaction results by block ID
	RemoveByBlockID(blockID flow.Identifier) error
}
