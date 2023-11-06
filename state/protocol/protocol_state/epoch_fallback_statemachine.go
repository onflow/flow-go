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

func (m *epochFallbackStateMachine) ProcessEpochSetup(epochSetup *flow.EpochSetup) (bool, error) {
	// won't process if we are in fallback mode
	return false, nil
}

func (m *epochFallbackStateMachine) ProcessEpochCommit(epochCommit *flow.EpochCommit) (bool, error) {
	// won't process if we are in fallback mode
	return false, nil
}

func (m *epochFallbackStateMachine) TransitionToNextEpoch() error {
	// won't process if we are in fallback mode
	return nil
}
