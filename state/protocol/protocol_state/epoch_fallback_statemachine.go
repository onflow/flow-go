package protocol_state

import (
	"github.com/onflow/flow-go/model/flow"
)

type epochFallbackStateMachine struct {
	baseProtocolStateMachine
}

var _ ProtocolStateMachine = (*epochFallbackStateMachine)(nil)

func newEpochFallbackStateMachine(view uint64, parentState *flow.RichProtocolStateEntry) *epochFallbackStateMachine {
	state := parentState.ProtocolStateEntry.Copy()
	state.InvalidStateTransitionAttempted = true
	return &epochFallbackStateMachine{
		baseProtocolStateMachine: baseProtocolStateMachine{
			parentState: parentState,
			state:       state,
			view:        view,
		},
	}
}

func (m *epochFallbackStateMachine) ProcessEpochSetup(_ *flow.EpochSetup) error {
	// won't process if we are in fallback mode
	return nil
}

func (m *epochFallbackStateMachine) ProcessEpochCommit(_ *flow.EpochCommit) error {
	// won't process if we are in fallback mode
	return nil
}

func (m *epochFallbackStateMachine) TransitionToNextEpoch() error {
	// won't process if we are in fallback mode
	return nil
}
