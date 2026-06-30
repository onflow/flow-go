package mock_interfaces

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/protocol_state/epochs"
)

// StateMachineFactoryMethod allows to create a mock for the StateMachineFactoryMethod callback
type StateMachineFactoryMethod interface {
	Execute(candidateView uint64, parentState *flow.RichEpochStateEntry) (epochs.StateMachine, error)
}
