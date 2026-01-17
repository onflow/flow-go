package optimistic_sync

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// Snapshot provides access to execution data readers for querying various data types from a specific ExecutionResult.
type Snapshot interface {
	// Events returns a reader for querying event data.
	Events() storage.EventsReader

	// LightTransactionResults returns a reader for querying light transaction result data.
	LightTransactionResults() storage.LightTransactionResultsReader

	// TransactionResultErrorMessages returns a reader for querying transaction error message data.
	TransactionResultErrorMessages() storage.TransactionResultErrorMessagesReader

	// Registers returns a reader for querying register data.
	//
	// Expected error returns during normal operation:
	//   - [indexer.ErrIndexNotInitialized]: If the storage is still bootstrapping.
	Registers() (storage.RegisterSnapshotReader, error)

	// BlockExecutionData returns a reader for querying execution data.
	BlockExecutionData() BlockExecutionDataReader

	// BlockStatus returns the block status for the block associated with the snapshot.
	BlockStatus() flow.BlockStatus
}
