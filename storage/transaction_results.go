package storage

import "github.com/dapperlabs/flow-go/model/flow"

// TransactionResult represents persistent storage for transaction result
type TransactionResults interface {

	// Store inserts the transaction result
	Store(blockID flow.Identifier, transactionResult *flow.TransactionResult) error

	// ByBlockIDTransactionID returns the transaction result for the given block ID and transaction ID
	ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.TransactionResult, error)
}
