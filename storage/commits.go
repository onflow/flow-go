package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

type CommitsReader interface {
	// ByBlockID will retrieve a commit by its ID from persistent storage.
	ByBlockID(blockID flow.Identifier) (flow.StateCommitment, error)
}

// Commits represents persistent storage for state commitments.
type Commits interface {
	CommitsReader

	// Store will store a commit in the persistent storage.
	Store(blockID flow.Identifier, commit flow.StateCommitment) error

	// BatchStore stores Commit keyed by blockID in provided batch
	// No errors are expected during normal operation, even if no entries are matched.
	// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchStore(blockID flow.Identifier, commit flow.StateCommitment, batch ReaderBatchWriter) error

	// BatchRemoveByBlockID removes Commit keyed by blockID in provided batch
	// No errors are expected during normal operation, even if no entries are matched.
	// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchRemoveByBlockID(blockID flow.Identifier, batch ReaderBatchWriter) error
}
