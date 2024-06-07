package epochs

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// DefaultEpochExtensionViewCount is a default length of epoch extension in views, approximately 1 day.
// TODO(EFM, #6020): replace this with value from KV store or protocol.GlobalParams
const DefaultEpochExtensionViewCount = 100_000

// FallbackStateMachine is a special structure that encapsulates logic for processing service events
// when protocol is in epoch fallback mode. The FallbackStateMachine ignores EpochSetup and EpochCommit
// events but still processes ejection events.
//
// Whenever invalid epoch state transition has been observed only epochFallbackStateMachines must be created for subsequent views.
// TODO(EFM, #6019): this structure needs to be updated to stop using parent state.
type FallbackStateMachine struct {
	baseStateMachine
	params protocol.GlobalParams
}

var _ StateMachine = (*FallbackStateMachine)(nil)

// NewFallbackStateMachine constructs a state machine for epoch fallback. It automatically sets
// EpochFallbackTriggered to true, thereby recording that we have entered epoch fallback mode.
// No errors are expected during normal operations.
func NewFallbackStateMachine(params protocol.GlobalParams, view uint64, parentState *flow.RichProtocolStateEntry) (*FallbackStateMachine, error) {
	state := parentState.ProtocolStateEntry.Copy()
	nextEpochCommitted := state.EpochPhase() == flow.EpochPhaseCommitted
	// we are entering fallback mode, this logic needs to be executed only once
	if !state.EpochFallbackTriggered {
		// the next epoch has not been committed, but possibly setup, make sure it is cleared
		if !nextEpochCommitted {
			state.NextEpoch = nil
		}
		state.EpochFallbackTriggered = true
	}

	sm := &FallbackStateMachine{
		baseStateMachine: baseStateMachine{
			parentState: parentState,
			state:       state,
			view:        view,
		},
		params: params,
	}

	if !nextEpochCommitted && view+params.EpochCommitSafetyThreshold() >= parentState.CurrentEpochFinalView() {
		// we have reached safety threshold and we are still in the fallback mode
		// prepare a new extension for the current epoch.
		err := sm.extendCurrentEpoch(flow.EpochExtension{
			FirstView:     parentState.CurrentEpochFinalView() + 1,
			FinalView:     parentState.CurrentEpochFinalView() + DefaultEpochExtensionViewCount, // TODO(EFM, #6020): replace with EpochExtensionLength
			TargetEndTime: 0,                                                                    // TODO(EFM, #6020): calculate and set target end time
		})
		if err != nil {
			return nil, err
		}
	}

	return sm, nil
}

// extendCurrentEpoch appends an epoch extension to the current epoch from underlying state.
// Internally, it performs sanity checks to ensure that the epoch extension is contiguous with the current epoch.
// It also ensures that the next epoch is not present, as epoch extensions are only allowed for the current epoch.
// No errors are expected during normal operation.
func (m *FallbackStateMachine) extendCurrentEpoch(epochExtension flow.EpochExtension) error {
	state := m.state
	if len(state.CurrentEpoch.EpochExtensions) > 0 {
		lastExtension := state.CurrentEpoch.EpochExtensions[len(state.CurrentEpoch.EpochExtensions)-1]
		if lastExtension.FinalView+1 != epochExtension.FirstView {
			return fmt.Errorf("epoch extension is not contiguous with the last extension")
		}
	} else {
		if epochExtension.FirstView != m.parentState.CurrentEpochSetup.FinalView+1 {
			return fmt.Errorf("first epoch extension is not contiguous with current epoch")
		}
	}

	if state.NextEpoch != nil {
		return fmt.Errorf("cannot extend current epoch when next epoch is present")
	}

	state.CurrentEpoch.EpochExtensions = append(state.CurrentEpoch.EpochExtensions, epochExtension)
	return nil
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

// ProcessEpochRecover updates the internally-maintained interim Epoch state with data from epoch recover
// event in an attempt to recover from Epoch Fallback Mode [EFM] and get back on happy path.
// Specifically, after successfully processing this event, we will have a next epoch (as specified by the
// EpochRecover event) in the protocol state, which is in the committed phase. Subsequently, the epoch
// protocol can proceed following the happy path. Therefore, we set `EpochFallbackTriggered` back to false.
//
// The boolean return indicates if the input event triggered a transition in the state machine or not.
// For the EpochRecover event, we return false if and only if there is an error. The reason is that
// either the `EpochRecover` event is rejected (leading to `InvalidServiceEventError`) or there is an
// exception processing the event. Otherwise, an `EpochRecover` event must always lead to a state change.
// TODO(EFM, #6018): this function needs to be updated to handle errors internally.
func (m *FallbackStateMachine) ProcessEpochRecover(epochRecover *flow.EpochRecover) (bool, error) {
	if m.view+m.params.EpochCommitSafetyThreshold() >= m.parentState.CurrentEpochFinalView() {
		return false, protocol.NewInvalidServiceEventErrorf("could not process epoch recover, safety threshold reached")
	}

	err := protocol.IsValidExtendingEpochSetup(&epochRecover.EpochSetup, m.parentState)
	if err != nil {
		return false, fmt.Errorf("invalid epoch recovery event(setup): %w", err)
	}

	err = protocol.IsValidEpochCommit(&epochRecover.EpochCommit, &epochRecover.EpochSetup)
	if err != nil {
		return false, fmt.Errorf("invalid epoch recovery event(commit): %w", err)
	}

	nextEpochParticipants, err := buildNextEpochActiveParticipants(
		m.parentState.CurrentEpoch.ActiveIdentities.Lookup(),
		m.parentState.CurrentEpochSetup,
		&epochRecover.EpochSetup)
	if err != nil {
		return false, fmt.Errorf("failed to build next epoch active participants: %w", err)
	}

	m.state.NextEpoch = &flow.EpochStateContainer{
		SetupID:          epochRecover.EpochSetup.ID(),
		CommitID:         epochRecover.EpochCommit.ID(),
		ActiveIdentities: nextEpochParticipants,
		EpochExtensions:  nil,
	}
	m.state.EpochFallbackTriggered = false
	return true, nil
}
