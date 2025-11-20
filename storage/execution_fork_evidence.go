package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

// ExecutionForkEvidence represents persistent storage for execution fork evidence.
// CAUTION: Not safe for concurrent use by multiple goroutines.
type ExecutionForkEvidence interface {
	// StoreIfNotExists stores the given conflictingSeals to the database
	// if no execution fork evidence is currently stored in the database.
	// This function is a no-op if evidence is already stored, because
	// only one execution fork evidence can be stored at a time.
	// The caller must hold the [storage.LockInsertExecutionForkEvidence] lock.
	// No errors are expected during normal operations.
	StoreIfNotExists(lctx lockctx.Proof, conflictingSeals []*flow.IncorporatedResultSeal) error

	// Retrieve reads conflicting seals from the database.
	// No error is returned if database record doesn't exist.
	// No errors are expected during normal operations.
	Retrieve() ([]*flow.IncorporatedResultSeal, error)
}
