package optimistic_sync

import (
	"github.com/onflow/flow-go/storage"
)

// Snapshot provides access to execution data readers for querying various data types from a specific ExecutionResult.
type Snapshot interface {
	// Events returns a reader for querying event data.
	Events() storage.EventsReader

	// Collections returns a reader for querying collection data.
	Collections() storage.CollectionsReader

	// Transactions returns a reader for querying transaction data.
	Transactions() storage.TransactionsReader

	// LightTransactionResults returns a reader for querying light transaction result data.
	LightTransactionResults() storage.LightTransactionResultsReader

	// TransactionResultErrorMessages returns a reader for querying transaction error message data.
	TransactionResultErrorMessages() storage.TransactionResultErrorMessagesReader

	// Registers returns a reader for querying register data.
	Registers() storage.RegisterIndexReader

	// BlockExecutionData returns a reader for querying execution data.
	BlockExecutionData() BlockExecutionDataReader
}
