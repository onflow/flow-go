package protocol_state

import (
	"github.com/onflow/flow-go/model/flow"
)

// epochFallbackStateMachine is a special structure that encapsulates logic for processing service events
// when protocol is in epoch fallback mode. Whenever invalid epoch state transition has been observed only
// epochFallbackStateMachine must be created for subsequent views.
// epochFallbackStateMachine ignores epoch setup and commit events but still processes ejection events.
type epochFallbackStateMachine struct {
	baseProtocolStateMachine
}

var _ ProtocolStateMachine = (*epochFallbackStateMachine)(nil)

// newEpochFallbackStateMachine constructs a state machine for epoch fallback, it automatically sets InvalidStateTransitionAttempted
// to true to mark that we have entered epoch fallback mode.
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
func (m *epochFallbackStateMachine) TransitionToNextEpoch() error {
	// won't process if we are in fallback mode
	return nil
}
