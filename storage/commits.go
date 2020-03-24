package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Commits represents persistent storage for state commitments.
type Commits interface {

	// Store will store a commit in the persistent storage.
	Store(blockID flow.Identifier, commit flow.StateCommitment) error

	// ByID will retrieve a commit by its ID from persistent storage.
	ByID(blockID flow.Identifier) (flow.StateCommitment, error)
}
