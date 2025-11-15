package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

type CommitsReader interface {
	// ByBlockID will retrieve a commit by its ID from persistent storage.
	ByBlockID(blockID flow.Identifier) (flow.StateCommitment, error)
}

// Commits represents persistent storage for state commitments.
type Commits interface {
	CommitsReader

	// BatchStore stores a state commitment keyed by the blockID whose execution results in that state.
	// The function ensures data integrity by first checking if a commitment already exists for the given block
	// and rejecting overwrites with different values. This function is idempotent, i.e. repeated calls with the
	// *initially* indexed value are no-ops.
	//
	// CAUTION:
	//   - Confirming that no value is already stored and the subsequent write must be atomic to prevent data corruption.
	//     The caller must acquire the [storage.LockIndexStateCommitment] and hold it until the database write has been committed.
	//
	// Expected error returns during normal operations:
	//   - [storage.ErrDataMismatch] if a *different* state commitment is already indexed for the same block ID
	BatchStore(lctx lockctx.Proof, blockID flow.Identifier, commit flow.StateCommitment, batch ReaderBatchWriter) error

	// BatchRemoveByBlockID removes Commit keyed by blockID in provided batch
	// No errors are expected during normal operation, even if no entries are matched.
	// If the database unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchRemoveByBlockID(blockID flow.Identifier, batch ReaderBatchWriter) error
}
