package protocol_state

import (
	"github.com/onflow/flow-go/model/flow"
)

// epochFallbackStateMachine is a special structure that encapsulates logic for processing service events
// when protocol is in epoch fallback mode. The epochFallbackStateMachine ignores EpochSetup and EpochCommit
// events but still processes ejection events.
//
// Whenever invalid epoch state transition has been observed only epochFallbackStateMachines must be created for subsequent views.
// TODO for 'leaving Epoch Fallback via special service event': this might need to change.
type epochFallbackStateMachine struct {
	baseProtocolStateMachine
}

var _ ProtocolStateMachine = (*epochFallbackStateMachine)(nil)

// newEpochFallbackStateMachine constructs a state machine for epoch fallback, it automatically sets
// InvalidEpochTransitionAttempted to true, thereby recording that we have entered epoch fallback mode.
func newEpochFallbackStateMachine(view uint64, parentState *flow.RichProtocolStateEntry) *epochFallbackStateMachine {
	state := parentState.ProtocolStateEntry.Copy()
	state.InvalidEpochTransitionAttempted = true
	return &epochFallbackStateMachine{
		baseProtocolStateMachine: baseProtocolStateMachine{
			parentState: parentState,
			state:       state,
			view:        view,
		},
	}
}

// ProcessEpochSetup processes epoch setup service events, for epoch fallback we are ignoring this event.
func (m *epochFallbackStateMachine) ProcessEpochSetup(_ *flow.EpochSetup) (bool, error) {
	// won't process if we are in fallback mode
	return false, nil
}

// ProcessEpochCommit processes epoch commit service events, for epoch fallback we are ignoring this event.
func (m *epochFallbackStateMachine) ProcessEpochCommit(_ *flow.EpochCommit) (bool, error) {
	// won't process if we are in fallback mode
	return false, nil
}

// TransitionToNextEpoch performs transition to next epoch, in epoch fallback no transitions are possible.
// TODO for 'leaving Epoch Fallback via special service event' this might need to change.
func (m *epochFallbackStateMachine) TransitionToNextEpoch() error {
	// won't process if we are in fallback mode
	return nil
}
