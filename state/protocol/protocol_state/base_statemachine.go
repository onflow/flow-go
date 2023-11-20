package protocol_state

import (
	"github.com/onflow/flow-go/state/protocol"

	"github.com/onflow/flow-go/model/flow"
)

// ProtocolStateMachine implements a low-level interface for state-changing operations on the protocol state.
// It is used by higher level logic to evolve the protocol state when certain events that are stored in blocks are observed.
// The ProtocolStateMachine is stateful and internally tracks the current protocol state. A separate instance is created for
// each block that is being processed.
type ProtocolStateMachine interface {
	// Build returns updated protocol state entry, state ID and a flag indicating if there were any changes.
	// CAUTION:
	// Do NOT call Build, if the ProtocolStateMachine instance has returned a `protocol.InvalidServiceEventError`
	// at any time during its lifetime. After this error, the ProtocolStateMachine is left with a potentially
	// dysfunctional state and should be discarded.
	Build() (updatedState *flow.ProtocolStateEntry, stateID flow.Identifier, hasChanges bool)

	// ProcessEpochSetup updates current protocol state with data from epoch setup event.
	// Processing epoch setup event also affects identity table for current epoch.
	// Observing an epoch setup event, transitions protocol state from staking to setup phase, we stop returning
	// identities from previous+current epochs and start returning identities from current+next epochs.
	// As a result of this operation protocol state for the next epoch will be created.
	// Returned boolean indicates if event triggered a transition in the state machine or not.
	// Implementors must never return (true, error).
	// Expected errors indicating that we are leaving the happy-path of the epoch transitions
	//   - `protocol.InvalidServiceEventError` - if the service event is invalid or is not a valid state transition for the current protocol state.
	//     CAUTION: the protocolStateMachine is left with a potentially dysfunctional state when this error occurs. Do NOT call the Build method
	//     after such error and discard the protocolStateMachine!
	ProcessEpochSetup(epochSetup *flow.EpochSetup) (bool, error)

	// ProcessEpochCommit updates current protocol state with data from epoch commit event.
	// Observing an epoch setup commit, transitions protocol state from setup to commit phase.
	// At this point, we have finished construction of the next epoch.
	// As a result of this operation protocol state for next epoch will be committed.
	// Returned boolean indicates if event triggered a transition in the state machine or not.
	// Implementors must never return (true, error).
	// Expected errors indicating that we are leaving the happy-path of the epoch transitions
	//   - `protocol.InvalidServiceEventError` - if the service event is invalid or is not a valid state transition for the current protocol state.
	//     CAUTION: the protocolStateMachine is left with a potentially dysfunctional state when this error occurs. Do NOT call the Build method
	//     after such error and discard the protocolStateMachine!
	ProcessEpochCommit(epochCommit *flow.EpochCommit) (bool, error)

	// EjectIdentity updates identity table by changing node's participation status to 'ejected'.
	// Should pass identity which is already present in the table, otherwise an exception will be raised.
	// Expected errors during normal operations:
	// - `protocol.InvalidServiceEventError` if the updated identity is not found in current and adjacent epochs.
	EjectIdentity(nodeID flow.Identifier) error

	// TransitionToNextEpoch discards current protocol state and transitions to the next epoch.
	// Epoch transition is only allowed when:
	// - next epoch has been set up,
	// - next epoch has been committed,
	// - candidate block is in the next epoch.
	// No errors are expected during normal operations.
	TransitionToNextEpoch() error

	// View returns the view that is associated with this ProtocolStateMachine.
	// The view of the ProtocolStateMachine equals the view of the block carrying the respective updates.
	View() uint64

	// ParentState returns parent protocol state that is associated with this ProtocolStateMachine.
	ParentState() *flow.RichProtocolStateEntry
}

// baseProtocolStateMachine implements common logic for evolving protocol state both in happy path and epoch fallback
// operation modes. It partially implements `ProtocolStateMachine` and is used as building block for more complex implementations.
type baseProtocolStateMachine struct {
	parentState *flow.RichProtocolStateEntry
	state       *flow.ProtocolStateEntry
	view        uint64

	// The following fields are maps from NodeID → DynamicIdentityEntry for the nodes that are *active* in the respective epoch.
	// Active means that these nodes are authorized to contribute to extending the chain. Formally, a node is active if and only
	// if it is listed in the EpochSetup event for the respective epoch. Note that map values are pointers, so writes to map values
	// will modify the respective DynamicIdentityEntry in `state`.

	prevEpochIdentitiesLookup    map[flow.Identifier]*flow.DynamicIdentityEntry // lookup for nodes active in the previous epoch, may be nil or empty
	currentEpochIdentitiesLookup map[flow.Identifier]*flow.DynamicIdentityEntry // lookup for nodes active in the current epoch, never nil or empty
	nextEpochIdentitiesLookup    map[flow.Identifier]*flow.DynamicIdentityEntry // lookup for nodes active in the next epoch, may be nil or empty
}

// Build returns updated protocol state entry, state ID and a flag indicating if there were any changes.
// CAUTION:
// Do NOT call Build, if the ProtocolStateMachine instance has returned a `protocol.InvalidServiceEventError`
// at any time during its lifetime. After this error, the ProtocolStateMachine is left with a potentially
// dysfunctional state and should be discarded.
func (u *baseProtocolStateMachine) Build() (updatedState *flow.ProtocolStateEntry, stateID flow.Identifier, hasChanges bool) {
	updatedState = u.state.Copy()
	stateID = updatedState.ID()
	hasChanges = stateID != u.parentState.ID()
	return
}

// View returns the view that is associated with this state protocolStateMachine.
// The view of the ProtocolStateMachine equals the view of the block carrying the respective updates.
func (u *baseProtocolStateMachine) View() uint64 {
	return u.view
}

// ParentState returns parent protocol state that is associated with this state protocolStateMachine.
func (u *baseProtocolStateMachine) ParentState() *flow.RichProtocolStateEntry {
	return u.parentState
}

// ensureLookupPopulated ensures that current and next epoch identities lookups are populated.
// We use this to avoid populating lookups on every UpdateIdentity call.
func (u *baseProtocolStateMachine) ensureLookupPopulated() {
	if len(u.currentEpochIdentitiesLookup) > 0 {
		return
	}
	u.rebuildIdentityLookup()
}

// rebuildIdentityLookup re-generates lookups of *active* participants for
// previous (optional, if u.state.PreviousEpoch ≠ nil), current (required) and
// next epoch (optional, if u.state.NextEpoch ≠ nil).
func (u *baseProtocolStateMachine) rebuildIdentityLookup() {
	if u.state.PreviousEpoch != nil {
		u.prevEpochIdentitiesLookup = u.state.PreviousEpoch.ActiveIdentities.Lookup()
	} else {
		u.prevEpochIdentitiesLookup = nil
	}
	u.currentEpochIdentitiesLookup = u.state.CurrentEpoch.ActiveIdentities.Lookup()
	if u.state.NextEpoch != nil {
		u.nextEpochIdentitiesLookup = u.state.NextEpoch.ActiveIdentities.Lookup()
	} else {
		u.nextEpochIdentitiesLookup = nil
	}
}

// EjectIdentity updates identity table by changing node's participation status to 'ejected'.
// Should pass identity which is already present in the table, otherwise an exception will be raised.
// Expected errors during normal operations:
// - `protocol.InvalidServiceEventError` if the updated identity is not found in current and adjacent epochs.
func (u *baseProtocolStateMachine) EjectIdentity(nodeID flow.Identifier) error {
	u.ensureLookupPopulated()
	prevEpochIdentity, foundInPrev := u.prevEpochIdentitiesLookup[nodeID]
	if foundInPrev {
		prevEpochIdentity.Ejected = true
	}
	currentEpochIdentity, foundInCurrent := u.currentEpochIdentitiesLookup[nodeID]
	if foundInCurrent {
		currentEpochIdentity.Ejected = true
	}
	nextEpochIdentity, foundInNext := u.nextEpochIdentitiesLookup[nodeID]
	if foundInNext {
		nextEpochIdentity.Ejected = true
	}
	if !foundInPrev && !foundInCurrent && !foundInNext {
		return protocol.NewInvalidServiceEventErrorf("expected to find identity for "+
			"prev, current or next epoch, but (%v) was not found", nodeID)
	}
	return nil
}
