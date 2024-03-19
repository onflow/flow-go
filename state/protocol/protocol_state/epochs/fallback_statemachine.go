package epochs

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
)

// FallbackStateMachine is a special structure that encapsulates logic for processing service events
// when protocol is in epoch fallback mode. The FallbackStateMachine ignores EpochSetup and EpochCommit
// events but still processes ejection events.
//
// Whenever invalid epoch state transition has been observed only epochFallbackStateMachines must be created for subsequent views.
// TODO for 'leaving Epoch Fallback via special service event': this might need to change.
type FallbackStateMachine struct {
	baseStateMachine
}

var _ protocol_state.EpochStateMachine = (*FallbackStateMachine)(nil)

// NewEpochFallbackStateMachine constructs a state machine for epoch fallback, it automatically sets
// InvalidEpochTransitionAttempted to true, thereby recording that we have entered epoch fallback mode.
func NewEpochFallbackStateMachine(view uint64, parentState *flow.RichProtocolStateEntry) *FallbackStateMachine {
	state := parentState.ProtocolStateEntry.Copy()
	state.InvalidEpochTransitionAttempted = true
	return &FallbackStateMachine{
		baseStateMachine: baseStateMachine{
			parentState: parentState,
			state:       state,
			view:        view,
		},
	}
}

// ProcessEpochSetup processes epoch setup service events, for epoch fallback we are ignoring this event.
func (m *FallbackStateMachine) ProcessEpochSetup(_ *flow.EpochSetup) (bool, error) {
	// won't process if we are in fallback mode
	return false, nil
}

// ProcessEpochCommit processes epoch commit service events, for epoch fallback we are ignoring this event.
func (m *FallbackStateMachine) ProcessEpochCommit(_ *flow.EpochCommit) (bool, error) {
	// won't process if we are in fallback mode
	return false, nil
}

// TransitionToNextEpoch performs transition to next epoch, in epoch fallback no transitions are possible.
// TODO for 'leaving Epoch Fallback via special service event' this might need to change.
func (m *FallbackStateMachine) TransitionToNextEpoch() error {
	// won't process if we are in fallback mode
	return nil
}
