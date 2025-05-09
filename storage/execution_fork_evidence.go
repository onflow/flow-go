package storage

import "github.com/onflow/flow-go/model/flow"

// ExecutionForkEvidence represents persistent storage for execution fork evidence.
type ExecutionForkEvidence interface {
	// StoreIfNotExists stores the given conflictingSeals to the database
	// if a record with the same key does not exist in the database.
	// If a record exists with the same key, this is a no-op.
	// No errors are expected during normal operations.
	StoreIfNotExists(conflictingSeals []*flow.IncorporatedResultSeal) error

	// Retrieve reads conflicting seals from the database.
	// No error is returned if database record doesn't exist.
	// No errors are expected during normal operations.
	Retrieve() ([]*flow.IncorporatedResultSeal, error)
}
