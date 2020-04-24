package storage

import "github.com/dapperlabs/flow-go/model/flow"

// TransactionErrors represents persistent storage for cadence errors that occurred during the execution of a transaction
type TransactionErrors interface {

	// Store inserts the transaction error
	Store(blockID flow.Identifier, transactionError *flow.TransactionError) error

	// ByBlockIDTransactionID returns the error for the given block ID and transaction ID
	ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.TransactionError, error)
}
