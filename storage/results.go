package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type ExecutionResultsReader interface {
	// ByID retrieves an execution result by its ID. Returns `ErrNotFound` if `resultID` is unknown.
	ByID(resultID flow.Identifier) (*flow.ExecutionResult, error)

	// ByIDTx returns a functor which retrieves the execution result by its ID, as part of a future database transaction.
	// When executing the functor, it returns `ErrNotFound` if no execution result with the respective ID is known.
	// deprecated
	ByIDTx(resultID flow.Identifier) func(*transaction.Tx) (*flow.ExecutionResult, error)

	// ByBlockID retrieves an execution result by block ID.
	ByBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error)
}

type ExecutionResults interface {
	ExecutionResultsReader

	// Store stores an execution result.
	Store(result *flow.ExecutionResult) error

	// BatchStore stores an execution result in a given batch
	BatchStore(result *flow.ExecutionResult, batch ReaderBatchWriter) error

	// Index indexes an execution result by block ID.
	Index(blockID flow.Identifier, resultID flow.Identifier) error

	// ForceIndex indexes an execution result by block ID overwriting existing database entry
	ForceIndex(blockID flow.Identifier, resultID flow.Identifier) error

	// BatchIndex indexes an execution result by block ID in a given batch
	BatchIndex(blockID flow.Identifier, resultID flow.Identifier, batch ReaderBatchWriter) error

	// BatchRemoveIndexByBlockID removes blockID-to-executionResultID index entries keyed by blockID in a provided batch.
	// No errors are expected during normal operation, even if no entries are matched.
	// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchRemoveIndexByBlockID(blockID flow.Identifier, batch ReaderBatchWriter) error
}
