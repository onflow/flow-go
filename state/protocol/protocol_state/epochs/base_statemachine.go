package epochs

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
)

// baseStateMachine implements common logic for evolving protocol state both in happy path and epoch fallback
// operation modes. It partially implements `StateMachine` and is used as building block for more complex implementations.
type baseStateMachine struct {
	telemetry   protocol_state.StateMachineTelemetryConsumer
	parentState *flow.RichEpochStateEntry
	state       *flow.EpochStateEntry
	ejector     ejector
	view        uint64
}

// newBaseStateMachine creates a new instance of baseStateMachine and performs initialization of the internal ejector
// which keeps track of ejected identities.
// A protocol.InvalidServiceEventError is returned if the ejector fails to track the identities.
func newBaseStateMachine(telemetry protocol_state.StateMachineTelemetryConsumer, view uint64, parentState *flow.RichEpochStateEntry, state *flow.EpochStateEntry) (*baseStateMachine, error) {
	ej := ejector{}
	if state.PreviousEpoch != nil {
		err := ej.TrackDynamicIdentityList(state.PreviousEpoch.ActiveIdentities)
		if err != nil {
			return nil, fmt.Errorf("could not track identities for previous epoch: %w", err)
		}
	}
	err := ej.TrackDynamicIdentityList(state.CurrentEpoch.ActiveIdentities)
	if err != nil {
		return nil, fmt.Errorf("could not track identities for current epoch: %w", err)
	}
	if state.NextEpoch != nil {
		err := ej.TrackDynamicIdentityList(state.NextEpoch.ActiveIdentities)
		if err != nil {
			return nil, fmt.Errorf("could not track identities for next epoch: %w", err)
		}
	}
	return &baseStateMachine{
		telemetry:   telemetry,
		view:        view,
		parentState: parentState,
		state:       state,
		ejector:     ej,
	}, nil
}

// Build returns updated protocol state entry, state ID and a flag indicating if there were any changes.
// CAUTION:
// Do NOT call Build, if the baseStateMachine instance has returned a `protocol.InvalidServiceEventError`
// at any time during its lifetime. After this error, the baseStateMachine is left with a potentially
// dysfunctional state and should be discarded.
func (u *baseStateMachine) Build() (updatedState *flow.EpochStateEntry, stateID flow.Identifier, hasChanges bool) {
	updatedState = u.state.Copy()
	stateID = updatedState.ID()
	hasChanges = stateID != u.parentState.ID()
	return
}

// View returns the view associated with this state machine.
// The view of the state machine equals the view of the block carrying the respective updates.
func (u *baseStateMachine) View() uint64 {
	return u.view
}

// ParentState returns parent protocol state associated with this state machine.
func (u *baseStateMachine) ParentState() *flow.RichEpochStateEntry {
	return u.parentState
}

// EjectIdentity updates identity table by changing the node's participation status to 'ejected'.
// Should pass identity which is already present in the table, otherwise an exception will be raised.
// Expected errors during normal operations:
// - `protocol.InvalidServiceEventError` if the updated identity is not found in current and adjacent epochs.
func (u *baseStateMachine) EjectIdentity(nodeID flow.Identifier) error {
	ejected := u.ejector.Eject(nodeID)
	if !ejected {
		return protocol.NewInvalidServiceEventErrorf("expected to find identity for "+
			"prev, current or next epoch, but (%v) was not found", nodeID)
	}
	return nil
}

// TransitionToNextEpoch updates the notion of 'current epoch', 'previous' and 'next epoch' in the protocol
// state. An epoch transition is only allowed when _all_ of the following conditions are satisfied:
// - next epoch has been set up,
// - next epoch has been committed,
// - candidate block is in the next epoch.
// No errors are expected during normal operations.
func (u *baseStateMachine) TransitionToNextEpoch() error {
	nextEpoch := u.state.NextEpoch
	if nextEpoch == nil { // nextEpoch ≠ nil if and only if next epoch was already set up (on the happy path)
		return fmt.Errorf("protocol state for next epoch has not yet been setup")
	}
	if nextEpoch.CommitID == flow.ZeroID { // nextEpoch.CommitID ≠ flow.ZeroID if and only if next epoch was already committed (on the happy path)
		return fmt.Errorf("protocol state for next epoch has not yet been committed")
	}
	// Check if we are at the next epoch, only then a transition is allowed
	if u.view < u.state.NextEpochSetup.FirstView {
		return fmt.Errorf("epoch transition is only allowed when entering next epoch")
	}
	u.state = &flow.EpochStateEntry{
		MinEpochStateEntry: &flow.MinEpochStateEntry{
			PreviousEpoch:          &u.state.CurrentEpoch,
			CurrentEpoch:           *u.state.NextEpoch,
			NextEpoch:              nil,
			EpochFallbackTriggered: u.state.EpochFallbackTriggered,
		},
		PreviousEpochSetup:  u.state.CurrentEpochSetup,
		PreviousEpochCommit: u.state.CurrentEpochCommit,
		CurrentEpochSetup:   u.state.NextEpochSetup,
		CurrentEpochCommit:  u.state.NextEpochCommit,
		NextEpochSetup:      nil,
		NextEpochCommit:     nil,
	}
	return nil
}
