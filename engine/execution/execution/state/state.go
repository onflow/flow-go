package state

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger"
)

// ExecutionState is an interface used to access and mutate the execution state of the blockchain.
type ExecutionState interface {
	// NewView creates a new ready-only view at the given state commitment.
	NewView(flow.StateCommitment) *View
	// CommitDelta commits a register delta and returns the new state commitment.
	CommitDelta(Delta) (flow.StateCommitment, error)
	// StateCommitmentByBlockHash returns the final state commitment for the provided block hash.
	StateCommitmentByBlockHash(crypto.Hash) (flow.StateCommitment, error)
}

type state struct {
	ls ledger.Storage
}

// NewExecutionState returns a new execution state access layer for the given ledger storage.
func NewExecutionState(ls ledger.Storage) ExecutionState {
	return &state{
		ls: ls,
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

func (s *state) StateCommitmentByBlockHash(crypto.Hash) (flow.StateCommitment, error) {
	// TODO: (post-MVP) get last state commitment from previous block
	// https://github.com/dapperlabs/flow-go/issues/2025
	return flow.StateCommitment(s.ls.LatestStateCommitment()), nil
}
