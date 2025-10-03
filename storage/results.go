package storage

import (
	"github.com/jordanschalm/lockctx"
	"github.com/onflow/flow-go/model/flow"
)

type ExecutionResultsReader interface {
	// ByID retrieves an execution result by its ID. Returns `ErrNotFound` if `resultID` is unknown.
	ByID(resultID flow.Identifier) (*flow.ExecutionResult, error)

	// ByBlockID retrieves an execution result by block ID.
	ByBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error)
}

type ExecutionResults interface {
	ExecutionResultsReader

	// Store stores an execution result.
	Store(result *flow.ExecutionResult) error

	// BatchStore stores an execution result in a given batch
	BatchStore(result *flow.ExecutionResult, batch ReaderBatchWriter) error

	// BatchIndex indexes an execution result by block ID in a given batch
	BatchIndex(lctx lockctx.Proof, blockID flow.Identifier, resultID flow.Identifier, batch ReaderBatchWriter) error

	// BatchRemoveIndexByBlockID removes blockID-to-executionResultID index entries keyed by blockID in a provided batch.
	// No errors are expected during normal operation, even if no entries are matched.
	// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchRemoveIndexByBlockID(blockID flow.Identifier, batch ReaderBatchWriter) error
}
