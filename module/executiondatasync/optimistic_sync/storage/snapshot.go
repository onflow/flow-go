package storage

import "github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/storage/reader"

// Snapshot provides access to execution data readers for querying various data types from a specific ExecutionResult.
type Snapshot interface {
	// Events returns a reader for querying event data.
	Events() reader.Events

	// Collections returns a reader for querying collection data.
	Collections() reader.Collections

	// Transactions returns a reader for querying transaction data.
	Transactions() reader.Transactions

	// LightTransactionResults returns a reader for querying light transaction result data.
	LightTransactionResults() reader.LightTransactionResults

	// TransactionResultErrorMessages returns a reader for querying transaction error message data.
	TransactionResultErrorMessages() reader.TransactionResultErrorMessages

	// Registers returns a reader for querying register data.
	Registers() reader.RegisterIndex
}
