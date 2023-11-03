package protocol_state

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
)

type epochFallbackStateMachine struct {
	baseProtocolStateMachine
}

var _ ProtocolStateMachine = (*epochFallbackStateMachine)(nil)

func newEpochFallbackStateMachine(view uint64, parentState *flow.RichProtocolStateEntry) (*epochFallbackStateMachine, error) {
	if !parentState.InvalidStateTransitionAttempted {
		return nil, fmt.Errorf("fallback state machine can be created only if we have already entered epoch fallback mode")
	}
	return &epochFallbackStateMachine{
		baseProtocolStateMachine: baseProtocolStateMachine{
			parentState: parentState,
			state:       parentState.ProtocolStateEntry.Copy(),
			view:        view,
		},
	}, nil
}

func transitionToEpochFallbackStateMachine(baseStateMachine ProtocolStateMachine) (*epochFallbackStateMachine, error) {
	parentState := baseStateMachine.ParentState()
	if parentState.InvalidStateTransitionAttempted {
		return nil, fmt.Errorf("could not create epoch fallback state machine as we are already in epoch fallback")
	}
	state, _, _ := baseStateMachine.Build()
	state.InvalidStateTransitionAttempted = true
	return &epochFallbackStateMachine{
		baseProtocolStateMachine{
			parentState: parentState,
			state:       state,
			view:        baseStateMachine.View(),
		},
	}, nil
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
