package epochs

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
)

// DefaultEpochExtensionViewCount is a default length of epoch extension in views, approximately 1 day.
// TODO(EFM, #6020): replace this with value from KV store or protocol.GlobalParams
const DefaultEpochExtensionViewCount = 100_000

// FallbackStateMachine is a special structure that encapsulates logic for processing service events
// when protocol is in epoch fallback mode. The FallbackStateMachine ignores EpochSetup and EpochCommit
// events but still processes ejection events.
//
// Whenever invalid epoch state transition has been observed only epochFallbackStateMachines must be created for subsequent views.
type FallbackStateMachine struct {
	baseStateMachine
	params protocol.GlobalParams
}

var _ StateMachine = (*FallbackStateMachine)(nil)

// NewFallbackStateMachine constructs a state machine for epoch fallback. It automatically sets
// EpochFallbackTriggered to true, thereby recording that we have entered epoch fallback mode.
// See flow.EpochPhase for detailed documentation about EFM and epoch phase transitions.
// No errors are expected during normal operations.
func NewFallbackStateMachine(params protocol.GlobalParams, telemetry protocol_state.StateMachineTelemetryConsumer, view uint64, parentState *flow.EpochRichStateEntry) (*FallbackStateMachine, error) {
	state := parentState.EpochStateEntry.Copy()
	nextEpochCommitted := state.EpochPhase() == flow.EpochPhaseCommitted
	// we are entering fallback mode, this logic needs to be executed only once
	if !state.EpochFallbackTriggered {
		// the next epoch has not been committed, but possibly setup, make sure it is cleared
		// CAUTION: this logic must be consistent with the `EpochMinStateEntry.EpochPhase()`, which
		// determines the epoch phase based on the configuration of the fields we set here!
		// Specifically, if and only if the next epoch is already committed as of the parent state,
		// we go through with that committed epoch. Otherwise, we have a tentative values of an epoch
		// not yet properly specified, which we have to clear out.
		if !nextEpochCommitted {
			state.NextEpoch = nil
		}
		state.EpochFallbackTriggered = true
	}

	sm := &FallbackStateMachine{
		baseStateMachine: baseStateMachine{
			telemetry:   telemetry,
			parentState: parentState,
			state:       state,
			view:        view,
		},
		params: params,
	}

	if !nextEpochCommitted && view+params.EpochCommitSafetyThreshold() >= state.CurrentEpochFinalView() {
		// we have reached safety threshold and we are still in the fallback mode
		// prepare a new extension for the current epoch.
		err := sm.extendCurrentEpoch(flow.EpochExtension{
			FirstView:     state.CurrentEpochFinalView() + 1,
			FinalView:     state.CurrentEpochFinalView() + DefaultEpochExtensionViewCount, // TODO(EFM, #6020): replace with EpochExtensionLength
			TargetEndTime: 0,                                                              // TODO(EFM, #6020): calculate and set target end time
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
		if epochExtension.FirstView != m.state.CurrentEpochSetup.FinalView+1 {
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
func (m *FallbackStateMachine) ProcessEpochSetup(setup *flow.EpochSetup) (bool, error) {
	m.telemetry.OnServiceEventReceived(setup.ServiceEvent())
	m.telemetry.OnInvalidServiceEvent(setup.ServiceEvent(), protocol.NewInvalidServiceEventErrorf("received EpochSetup in Epoch Fallback Mode"))
	// Note that we are dropping _all_ EpochSetup events sealed by this block. As long as we are in EFM, this is
	// the natural behaviour, as we have given up on following the instructions from the Epoch Smart Contracts.
	//
	// CAUTION: This leaves an edge case where, a valid `EpochRecover` event followed by an `EpochSetup` is sealed in the
	// same block. Conceptually, this is a clear indication that the Epoch Smart contract is doing something unexpect. The
	// reason is that the block with the `EpochRecover` event is at least `EpochCommitSafetyThreshold` views before the
	// switchover to the recovery epoch. Otherwise, the FallbackStateMachine constructor would have added an extension to
	// the current epoch. Axiomatically, the `EpochCommitSafetyThreshold` is large enough that we guarantee finalization of
	// the epoch configuration (in this case the configuration of the recovery epoch provided by the `EpochRecover` event)
	// _before_ the recovery epoch starts. For finalization, the block sealing the `EpochRecover` event must have descendants
	// in the same epoch, i.e. an EpochSetup cannot occur in the same block as the `EpochRecover` event.
	//
	// Nevertheless, we ignore such an EpochSetup event here, despite knowing that it is an invalid input from the smart contract.
	// If the epoch smart contract continues to behave unexpectedly, we will just re-enter EFM in a subsequent block. Though,
	// if the smart contract happens to behave as expected for all subsequent blocks and manages to coordinate epoch transitions
	// from here on, that is also acceptable.
	// Essentially, the block sealing a valid EpochRecover event is a grace period, where we still tolerate unexpected events from
	// the Epoch Smart Contract. This significantly simplifies the implementation of the FallbackStateMachine without impacting the
	// robustness of the overall EFM mechanics.
	return false, nil
}

// ProcessEpochCommit processes epoch commit service events, for epoch fallback we are ignoring this event.
func (m *FallbackStateMachine) ProcessEpochCommit(setup *flow.EpochCommit) (bool, error) {
	m.telemetry.OnServiceEventReceived(setup.ServiceEvent())
	m.telemetry.OnInvalidServiceEvent(setup.ServiceEvent(), protocol.NewInvalidServiceEventErrorf("received EpochCommit in Epoch Fallback Mode"))
	// We ignore _all_ EpochCommit events here. This includes scenarios where a valid `EpochRecover` event is sealed in
	// a block followed by `EpochSetup` and/or `EpochCommit` events -- technically, a clear indication that the Epoch Smart
	// contract is doing something unexpected. For a detailed explanation why this is safe, see `ProcessEpochSetup` above.
	return false, nil
}

// ProcessEpochRecover updates the internally-maintained interim Epoch state with data from epoch recover
// event in an attempt to recover from Epoch Fallback Mode [EFM] and get back on happy path.
// Specifically, after successfully processing this event, we will have a next epoch (as specified by the
// EpochRecover event) in the protocol state, which is in the committed phase. Subsequently, the epoch
// protocol can proceed following the happy path. Therefore, we set `EpochFallbackTriggered` back to false.
//
// The boolean return indicates if the input event triggered a transition in the state machine or not.
// For the EpochRecover event, we never return an error to ensure that FallbackStateMachine is robust against any input and doesn't
// halt the chain even if the Epoch Smart Contract misbehaves. This is a safe choice since the error can only originate from
// an invalid EpochRecover event, in this case we just ignore the event and continue with the fallback mode.
//
// EDGE CASES: due to manual interventions for Epoch Recovery, there is a notable risk of unintended side-effects
// in terms of emitted events. Therefore, we aim to be resilient against invalid and/or inconsistent events:
//  1. Any amount of setup and commit events being sealed in the same block as an epoch recover event:
//     EpochSetup and EpochCommit are consistently ignored by the FallbackStateMachine, also after a successful recovery.
//  2. Multiple EpochRecover events sealed in the same block:
//     - Invalid `EpochRecover` events are reported to telemetry and dropped.
//     - The first valid `EpochRecover` event is accepted (if any is sealed in block)
//     - Subsequent valid events are no-ops iff they are identical to the first valid EpochRecover event.
//     Otherwise, they are reported to telemetry and dropped.
//     An `EpochRecover` event is considered valid in this context if it specifies a valid successor of the
//     current epoch (irrespective whether a `NextEpoch` in the `ProtocolStateEntry`)
//
// Error returns:
//   - During normal operations, this method internally handle erroneous inputs. Error returns are
//     symptoms of internal state corruption or critical bugs, making continuation impossible.
func (m *FallbackStateMachine) ProcessEpochRecover(epochRecover *flow.EpochRecover) (bool, error) {
	m.telemetry.OnServiceEventReceived(epochRecover.ServiceEvent())
	err := m.ensureValidEpochRecover(epochRecover)
	if err != nil {
		m.telemetry.OnInvalidServiceEvent(epochRecover.ServiceEvent(), err)
		return false, nil
	}

	nextEpochParticipants, err := buildNextEpochActiveParticipants(
		// TOTO: The following usage of the _parent_ state Active identities might lose ejections
		//       sealed in this block. See https://github.com/onflow/flow-go/issues/6019
		m.state.CurrentEpoch.ActiveIdentities.Lookup(),
		m.state.CurrentEpochSetup,
		&epochRecover.EpochSetup)
	if err != nil {
		m.telemetry.OnInvalidServiceEvent(epochRecover.ServiceEvent(), err)
		return false, nil
	}

	nextEpoch := m.state.NextEpoch
	if nextEpoch == nil {
		// setup next epoch if there is none
		m.state.NextEpoch = &flow.EpochStateContainer{
			SetupID:          epochRecover.EpochSetup.ID(),
			CommitID:         epochRecover.EpochCommit.ID(),
			ActiveIdentities: nextEpochParticipants,
			EpochExtensions:  nil,
		}
	} else {
		// accept iff the EpochRecover is the same as the one we have already recovered.
		if nextEpoch.SetupID != epochRecover.EpochSetup.ID() ||
			nextEpoch.CommitID != epochRecover.EpochCommit.ID() {
			m.telemetry.OnInvalidServiceEvent(epochRecover.ServiceEvent(),
				protocol.NewInvalidServiceEventErrorf("multiple inconsistent EpochRecover events sealed in the same block"))
			return false, nil
		}
	}
	// if we have processed a valid EpochRecover event, we should exit EFM.
	m.state.EpochFallbackTriggered = false
	m.telemetry.OnServiceEventProcessed(epochRecover.ServiceEvent())
	return true, nil
}

// ensureValidEpochRecover performs validity checks on the epoch recover event.
// Expected errors during normal operations:
// * `protocol.InvalidServiceEventError` - if the service event is invalid or is not a valid state transition for the current protocol state.
// This is a side-effect-free function. This function only returns protocol.InvalidServiceEventError as errors.
func (m *FallbackStateMachine) ensureValidEpochRecover(epochRecover *flow.EpochRecover) error {
	if m.view+m.params.EpochCommitSafetyThreshold() >= m.state.CurrentEpochFinalView() {
		return protocol.NewInvalidServiceEventErrorf("could not process epoch recover, safety threshold reached")
	}
	// TOTO: The following code is only safe if the parent state has the _identical_ `CurrentEpochFinalView` as the
	//       evolving state. This is regularly violated at the block we insert the epoch extension.
	//       see https://github.com/onflow/flow-go/issues/6019
	err := protocol.IsValidExtendingEpochSetup(&epochRecover.EpochSetup, m.state)
	if err != nil {
		return fmt.Errorf("invalid epoch recovery event(setup): %w", err)
	}
	err = protocol.IsValidEpochCommit(&epochRecover.EpochCommit, &epochRecover.EpochSetup)
	if err != nil {
		return fmt.Errorf("invalid epoch recovery event(commit): %w", err)
	}
	return nil
}
