package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// StateCommitments represents persistent storage for state commitments.
type StateCommitments interface {
	Persist(id flow.Identifier, stateCommitment *flow.StateCommitment) error

	ByID(id flow.Identifier) (*flow.StateCommitment, error)
}
