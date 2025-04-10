package storage

import "github.com/onflow/flow-go/model/flow"

// TransactionResultErrorMessagesReader represents persistent storage read operations for transaction result error messages
type TransactionResultErrorMessagesReader interface {
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

// TransactionResultErrorMessages represents persistent storage for transaction result error messages
type TransactionResultErrorMessages interface {
	TransactionResultErrorMessagesReader

	// Store will store transaction result error messages for the given block ID.
	//
	// No errors are expected during normal operation.
	Store(blockID flow.Identifier, transactionResultErrorMessages []flow.TransactionResultErrorMessage) error
}
