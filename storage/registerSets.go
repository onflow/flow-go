package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// RegisterSets represents persistent storage for register sets.
type RegisterSets interface {

	// Store will store a register set in the persistent storage.
	Store(commit flow.StateCommitment, set *flow.RegisterSet) error

	// ByCommit will retrieve a register set by its state commitment from persistent storage.
	ByCommit(commit flow.StateCommitment) (*flow.RegisterSet, error)
}
