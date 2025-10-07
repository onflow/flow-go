package store

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// ExecutionForkEvidence represents persistent storage for execution fork evidence.
type ExecutionForkEvidence struct {
	db storage.DB
}

var _ storage.ExecutionForkEvidence = (*ExecutionForkEvidence)(nil)

// NewExecutionForkEvidence creates a new ExecutionForkEvidence store.
func NewExecutionForkEvidence(db storage.DB) *ExecutionForkEvidence {
	return &ExecutionForkEvidence{db: db}
}

// StoreIfNotExists stores the given conflictingSeals to the database.
// This is a no-op if there is already a record in the database with the same key.
// The caller must hold the [storage.LockInsertExecutionForkEvidence] lock.
// No errors are expected during normal operations.
func (efe *ExecutionForkEvidence) StoreIfNotExists(lctx lockctx.Proof, conflictingSeals []*flow.IncorporatedResultSeal) error {
	return efe.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.InsertExecutionForkEvidence(lctx, rw, conflictingSeals)
	})
}

// Retrieve reads conflicting seals from the database.
// No error is returned if database record doesn't exist.
// No errors are expected during normal operations.
func (efe *ExecutionForkEvidence) Retrieve() ([]*flow.IncorporatedResultSeal, error) {
	var conflictingSeals []*flow.IncorporatedResultSeal
	err := operation.RetrieveExecutionForkEvidence(efe.db.Reader(), &conflictingSeals)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, nil // No evidence in the database.
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load evidence whether or not an execution fork occured: %w", err)
	}
	return conflictingSeals, nil
}
