package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

type ExecutionResultsReader interface {
	// ByID retrieves an execution result by its ID. Returns [storage.ErrNotFound] if `resultID` is unknown.
	ByID(resultID flow.Identifier) (*flow.ExecutionResult, error)

	// ByBlockID retrieves an execution result by block ID.
	// It returns [storage.ErrNotFound] if `blockID` refers to a block which is unknown, or for which a trusted (sealed or executed by this node) execution result does not exist.
	ByBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error)

	// IDByBlockID retrieves an execution result ID by block ID.
	// It returns [storage.ErrNotFound] if `blockID` does not refer to a block executed by this node
	IDByBlockID(blockID flow.Identifier) (flow.Identifier, error)
}

type ExecutionResults interface {
	ExecutionResultsReader

	// BatchStore stores an execution result in a given batch.
	// The key (result ID) is derived from the value (result) via a collision-resistant hash function. Hence,
	// unchecked overwrites pose no risk of data corruption, because for the same key, we expect the same value.
	// No error is expected during normal operation.
	BatchStore(result *flow.ExecutionResult, batch ReaderBatchWriter) error

	// BatchIndex indexes an execution result by block ID in a given batch.
	// Conceptually, an execution result for a block should be persisted once and never changed thereafter.
	// The function enforces this, for which reason the caller must acquire [storage.LockIndexExecutionResult].
	// It returns [storage.ErrDataMismatch] if there is already an indexed result for the given blockID,
	// but it is different from the given resultID.
	BatchIndex(lctx lockctx.Proof, rw ReaderBatchWriter, blockID flow.Identifier, resultID flow.Identifier) error

	// BatchRemoveIndexByBlockID removes blockID-to-executionResultID index entries keyed by blockID in a provided batch.
	// No errors are expected during normal operation, even if no entries are matched.
	BatchRemoveIndexByBlockID(blockID flow.Identifier, batch ReaderBatchWriter) error
}
