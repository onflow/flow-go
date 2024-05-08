package epochs

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// DefaultEpochExtensionLength is a default length of epoch extension.
// TODO(efm-recovery): replace this with value from KV store or protocol.GlobalParams
const DefaultEpochExtensionLength = 1_000

// FallbackStateMachine is a special structure that encapsulates logic for processing service events
// when protocol is in epoch fallback mode. The FallbackStateMachine ignores EpochSetup and EpochCommit
// events but still processes ejection events.
//
// Whenever invalid epoch state transition has been observed only epochFallbackStateMachines must be created for subsequent views.
// TODO for 'leaving Epoch Fallback via special service event': this might need to change.
type FallbackStateMachine struct {
	baseStateMachine
}

var _ StateMachine = (*FallbackStateMachine)(nil)

// NewFallbackStateMachine constructs a state machine for epoch fallback, it automatically sets
// InvalidEpochTransitionAttempted to true, thereby recording that we have entered epoch fallback mode.
// No errors are expected during normal operations.
func NewFallbackStateMachine(params protocol.GlobalParams, view uint64, parentState *flow.RichProtocolStateEntry) (*FallbackStateMachine, error) {
	state := parentState.ProtocolStateEntry.Copy()
	nextEpochCommitted := state.EpochPhase() == flow.EpochPhaseCommitted
	// we are entering fallback mode, this logic needs to be executed only once
	if !state.InvalidEpochTransitionAttempted {
		// the next epoch has not been committed, but possibly setup, make sure it is cleared
		if !nextEpochCommitted {
			state.NextEpoch = nil
		}
		state.InvalidEpochTransitionAttempted = true
	}

	if !nextEpochCommitted && view+params.EpochCommitSafetyThreshold() >= parentState.CurrentEpochFinalView() {
		// we have reached safety threshold and we are still in the fallback mode
		// prepare a new extension for the current epoch.
		err := state.CurrentEpoch.ExtendEpoch(flow.EpochExtension{
			FirstView:     parentState.CurrentEpochFinalView() + 1,
			FinalView:     parentState.CurrentEpochFinalView() + 1 + DefaultEpochExtensionLength, // TODO: replace with EpochExtensionLength
			TargetEndTime: 0,                                                                     // TODO: calculate and set target end time
		})
		if err != nil {
			return nil, fmt.Errorf("could not produce a valid epoch extension: %w", err)
		}
	}

	return &FallbackStateMachine{
		baseStateMachine: baseStateMachine{
			parentState: parentState,
			state:       state,
			view:        view,
		},
	}, nil
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
