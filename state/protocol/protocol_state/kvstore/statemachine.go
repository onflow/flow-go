package kvstore

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
)

type ProcessingStateMachine struct {
	view uint64
}

var _ protocol_state.KeyValueStoreStateMachine = (*ProcessingStateMachine)(nil)

func NewProcessingStateMachine(view uint64) *ProcessingStateMachine {
	return &ProcessingStateMachine{
		view: view,
	}
}

func (ProcessingStateMachine) Build() (updatedState protocol_state.Reader, stateID flow.Identifier, hasChanges bool) {
	//TODO implement me
	panic("implement me")
}

func (ProcessingStateMachine) ProcessUpdate(update *flow.ServiceEvent) error {
	//TODO implement me
	panic("implement me")
}

func (ProcessingStateMachine) View() uint64 {
	//TODO implement me
	panic("implement me")
}

func (ProcessingStateMachine) ParentState() protocol_state.Reader {
	//TODO implement me
	panic("implement me")
}
