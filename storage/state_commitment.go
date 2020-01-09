package storage

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// StateCommitments represents persistent storage for state commitments.
type StateCommitments interface {
	Persist(hash crypto.Hash, stateCommitment *flow.StateCommitment) error

	ByHash(hash crypto.Hash) (*flow.StateCommitment, error)
}
