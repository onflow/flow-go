package state

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger"
)

type ExecutionState interface {
	NewView(flow.StateCommitment) *View
	CommitDelta(Delta) (flow.StateCommitment, error)
}

type state struct {
	ls ledger.Storage
}

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
	updates := delta.Updates()

	ids := make([]ledger.RegisterID, 0, len(updates))
	values := make([]ledger.RegisterValue, 0, len(updates))

	for id, value := range delta.Updates() {
		ids = append(ids, ledger.RegisterID(id))
		values = append(values, value)
	}

	commitment, _, err := s.ls.UpdateRegistersWithProof(ids, values)
	if err != nil {
		return nil, err
	}

	return flow.StateCommitment(commitment), nil
}
