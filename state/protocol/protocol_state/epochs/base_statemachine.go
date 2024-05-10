package epochs

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// baseStateMachine implements common logic for evolving protocol state both in happy path and epoch fallback
// operation modes. It partially implements `StateMachine` and is used as building block for more complex implementations.
type baseStateMachine struct {
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
// Do NOT call Build, if the baseStateMachine instance has returned a `protocol.InvalidServiceEventError`
// at any time during its lifetime. After this error, the baseStateMachine is left with a potentially
// dysfunctional state and should be discarded.
func (u *baseStateMachine) Build() (updatedState *flow.ProtocolStateEntry, stateID flow.Identifier, hasChanges bool) {
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
func (u *baseStateMachine) ParentState() *flow.RichProtocolStateEntry {
	return u.parentState
}

// ensureLookupPopulated ensures that current and next epoch identities lookups are populated.
// We use this to avoid populating lookups on every UpdateIdentity call.
func (u *baseStateMachine) ensureLookupPopulated() {
	if len(u.currentEpochIdentitiesLookup) > 0 {
		return
	}
	u.rebuildIdentityLookup()
}

// rebuildIdentityLookup re-generates lookups of *active* participants for
// previous (optional, if u.state.PreviousEpoch ≠ nil), current (required) and
// next epoch (optional, if u.state.NextEpoch ≠ nil).
func (u *baseStateMachine) rebuildIdentityLookup() {
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

// EjectIdentity updates identity table by changing the node's participation status to 'ejected'.
// Should pass identity which is already present in the table, otherwise an exception will be raised.
// Expected errors during normal operations:
// - `protocol.InvalidServiceEventError` if the updated identity is not found in current and adjacent epochs.
func (u *baseStateMachine) EjectIdentity(nodeID flow.Identifier) error {
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

// TransitionToNextEpoch updates the notion of 'current epoch', 'previous' and 'next epoch' in the protocol
// state. An epoch transition is only allowed when:
// - next epoch has been set up,
// - next epoch has been committed,
// - invalid state transition has not been attempted (this is ensured by constructor),
// - candidate block is in the next epoch.
// No errors are expected during normal operations.
func (u *baseStateMachine) TransitionToNextEpoch() error {
	nextEpoch := u.state.NextEpoch
	// Check if there is next epoch protocol state
	if nextEpoch == nil {
		return fmt.Errorf("protocol state has not been setup yet")
	}
	// Check if there is a commit event for next epoch
	if nextEpoch.CommitID == flow.ZeroID {
		return fmt.Errorf("protocol state has not been committed yet")
	}
	// Check if we are at the next epoch, only then a transition is allowed
	if u.view < u.parentState.NextEpochSetup.FirstView {
		return fmt.Errorf("protocol state transition is only allowed when enterring next epoch")
	}
	u.state = &flow.ProtocolStateEntry{
		PreviousEpoch:                   &u.state.CurrentEpoch,
		CurrentEpoch:                    *u.state.NextEpoch,
		InvalidEpochTransitionAttempted: u.state.InvalidEpochTransitionAttempted,
	}
	u.rebuildIdentityLookup()
	return nil
}
