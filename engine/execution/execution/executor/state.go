package executor

import (
	"github.com/dapperlabs/flow-go/engine/execution/execution/ledger"
	"github.com/dapperlabs/flow-go/model/flow"
)

type State interface {
	NewView() *ledger.View
	ApplyDelta(delta ledger.Delta) flow.StateCommitment
}

type state struct{}

func NewState() State {
	return &state{}
}

func (s *state) NewView() *ledger.View {
	return ledger.NewView(func(key string) (bytes []byte, e error) {
		return nil, nil
	})
}

func (s *state) ApplyDelta(delta ledger.Delta) flow.StateCommitment {
	// TODO: commit chunk to storage - https://github.com/dapperlabs/flow-go/issues/1915
	return nil
}
