package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type ExecutionResults interface {

	// Store stores an execution result.
	Store(result *flow.ExecutionResult) error

	// BatchStore stores an execution result in a given batch
	BatchStore(result *flow.ExecutionResult, batch BatchStorage) error

	// ByID retrieves an execution result by its ID.
	ByID(resultID flow.Identifier) (*flow.ExecutionResult, error)

	// ByIDTx retrieves an execution result by its ID in the context of the given transaction
	ByIDTx(resultID flow.Identifier) func(*transaction.Tx) (*flow.ExecutionResult, error)

	// Index indexes an execution result by block ID.
	Index(blockID flow.Identifier, resultID flow.Identifier) error

	// ForceIndex indexes an execution result by block ID overwriting existing database entry
	ForceIndex(blockID flow.Identifier, resultID flow.Identifier) error

	// BatchIndex indexes an execution result by block ID in a given batch
	BatchIndex(blockID flow.Identifier, resultID flow.Identifier, batch BatchStorage) error

	// ByBlockID retrieves an execution result by block ID.
	ByBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error)

	// BatchRemoveIndexByBlockID removes blockID-to-executionResultID index entries keyed by blockID in a provided batch.
	// No errors are expected during normal operation, even if no entries are matched.
	// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchRemoveIndexByBlockID(blockID flow.Identifier, batch BatchStorage) error
}
