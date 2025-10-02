package optimistic_sync

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	execdatacache "github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
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

	// ExecutionData returns a reader for querying execution data.
	ExecutionData() execdatacache.ExecutionDataCache

	//BlockExecutionData returns a reader for querying execution data.
	BlockExecutionData(ctx context.Context, blockID flow.Identifier) execution_data.BlockExecutionData
}
