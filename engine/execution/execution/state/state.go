package state

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/ledger"
)

// ExecutionState is an interface used to access and mutate the execution state of the blockchain.
type ExecutionState interface {
	// NewView creates a new ready-only view at the given state commitment.
	NewView(flow.StateCommitment) *View
	// CommitDelta commits a register delta and returns the new state commitment.
	CommitDelta(Delta) (flow.StateCommitment, error)
	// StateCommitmentByBlockID returns the final state commitment for the provided block ID.
	StateCommitmentByBlockID(flow.Identifier) (flow.StateCommitment, error)

	// PersistStateCommitment saves a state commitment under given hash
	PersistStateCommitment(flow.Identifier, *flow.StateCommitment) error
}

type state struct {
	ls          ledger.Storage
	commitments storage.StateCommitments
}

// NewExecutionState returns a new execution state access layer for the given ledger storage.
func NewExecutionState(ls ledger.Storage, commitments storage.StateCommitments) ExecutionState {
	return &state{
		ls:          ls,
		commitments: commitments,
	}
}

func (s *state) NewView(commitment flow.StateCommitment) *View {
	return NewView(func(key string) ([]byte, error) {
		values, err := s.ls.GetRegisters(
			[]ledger.RegisterID{[]byte(key)},
			ledger.StateCommitment(commitment),
		)
		if err != nil {
			return nil, err
		}

		if len(values) == 0 {
			return nil, nil
		}

		return values[0], nil
	})
}

func (s *state) CommitDelta(delta Delta) (flow.StateCommitment, error) {
	ids, values := delta.RegisterUpdates()

	// TODO: update CommitDelta to also return proofs
	commitment, _, err := s.ls.UpdateRegistersWithProof(ids, values)
	if err != nil {
		return nil, err
	}

	return flow.StateCommitment(commitment), nil
}

func (s *state) StateCommitmentByBlockID(id flow.Identifier) (flow.StateCommitment, error) {
	commitment, err := s.commitments.ByID(id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			//TODO ? Shouldn't happen in MVP, in multi-node should query a state from other nodes
			panic(fmt.Sprintf("storage commitment for id %v does not exist", id))
		} else {
			return nil, err
		}
	}
	return *commitment, nil
}

func (s *state) PersistStateCommitment(id flow.Identifier, commitment *flow.StateCommitment) error {
	return s.commitments.Persist(id, commitment)
}
