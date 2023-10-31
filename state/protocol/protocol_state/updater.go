package protocol_state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/state/protocol"
)

// stateMachine is a dedicated structure that encapsulates all logic for updating protocol state.
// Only protocol state updater knows how to update protocol state in a way that is consistent with the protocol.
// Protocol state updater implements the following state changes:
// - epoch setup: transitions current epoch from staking to setup phase, creates next epoch protocol state when processed.
// - epoch commit: transitions current epoch from setup to commit phase, commits next epoch protocol state when processed.
// - identity changes: updates identity table for current and next epoch(if available).
// - setting an invalid state transition flag: sets an invalid state transition flag for current epoch and next epoch(if available).
// All updates are applied to a copy of parent protocol state, so parent protocol state is not modified.
// It is NOT safe to use in concurrent environment.
type stateMachine struct {
	parentState *flow.RichProtocolStateEntry
	state       *flow.ProtocolStateEntry
	view        uint64

	// The following fields are maps from NodeID → DynamicIdentityEntry for the nodes that are *active* in the respective epoch.
	// Active means that these nodes are authorized to contribute to extending the chain. Formally, a node is active if and only
	// if it is listed in the EpochSetup event for the respective epoch. Note that map values are pointers, so writes to map values
	// will modify the respective DynamicIdentityEntry in EpochStateContainer.

	prevEpochIdentitiesLookup    map[flow.Identifier]*flow.DynamicIdentityEntry // lookup for nodes active in the previous epoch, may be nil or empty
	currentEpochIdentitiesLookup map[flow.Identifier]*flow.DynamicIdentityEntry // lookup for nodes active in the current epoch, never nil or empty
	nextEpochIdentitiesLookup    map[flow.Identifier]*flow.DynamicIdentityEntry // lookup for nodes active in the next epoch, may be nil or empty
}

var _ ProtocolStateMachine = (*stateMachine)(nil)

// newStateMachine creates a new protocol state updater.
func newStateMachine(view uint64, parentState *flow.RichProtocolStateEntry) *stateMachine {
	updater := &stateMachine{
		parentState: parentState,
		state:       parentState.ProtocolStateEntry.Copy(),
		view:        view,
	}
	return updater
}

// Build returns updated protocol state entry, state ID and a flag indicating if there were any changes.
func (u *stateMachine) Build() (updatedState *flow.ProtocolStateEntry, stateID flow.Identifier, hasChanges bool) {
	updatedState = u.state.Copy()
	stateID = updatedState.ID()
	hasChanges = stateID != u.parentState.ID()
	return
}

// ProcessEpochSetup updates the protocol state with data from the epoch setup event.
// Observing an epoch setup event also affects the identity table for current epoch:
//   - it transitions the protocol state from Staking to Epoch Setup phase
//   - we stop returning identities from previous+current epochs and instead returning identities from current+next epochs.
//
// As a result of this operation protocol state for the next epoch will be created.
// Expected errors during normal operations:
// - `protocol.InvalidServiceEventError` if the service event is invalid or is not a valid state transition for the current protocol state
func (u *stateMachine) ProcessEpochSetup(epochSetup *flow.EpochSetup) error {
	if u.state.InvalidStateTransitionAttempted {
		return nil // won't process new events if we are in epoch fallback mode.
	}
	err := protocol.IsValidExtendingEpochSetup(epochSetup, u.parentState.CurrentEpochSetup, u.parentState.EpochStatus())
	if err != nil {
		return fmt.Errorf("invalid epoch setup event: %w", err)
	}
	if u.state.NextEpoch != nil {
		return protocol.NewInvalidServiceEventErrorf("repeated setup for epoch %d", epochSetup.Counter)
	}

	// When observing setup event for subsequent epoch, construct the EpochStateContainer for `ProtocolStateEntry.NextEpoch`.
	// Context:
	// Note that the `EpochStateContainer.ActiveIdentities` only contains the nodes that are *active* in the next epoch. Active means
	// that these nodes are authorized to contribute to extending the chain. Nodes are listed in `ActiveIdentities` if and only if
	// they are part of the EpochSetup event for the respective epoch.
	//
	// sanity checking SAFETY-CRITICAL INVARIANT (I):
	//   - Per convention, the `flow.EpochSetup` event should list the IdentitySkeletons in canonical order. This is useful
	//     for most efficient construction of the full active Identities for an epoch. We enforce this here at the gateway
	//     to the protocol state, when we incorporate new information from the EpochSetup event.
	//   - Note that the system smart contracts manage the identity table as an unordered set! For the protocol state, we desire a fixed
	//     ordering to simplify various implementation details, like the DKG. Therefore, we order identities in `flow.EpochSetup` during
	//     conversion from cadence to Go in the function `convert.ServiceEvent(flow.ChainID, flow.Event)` in package `model/convert`
	// sanity checking SAFETY-CRITICAL INVARIANT (II):
	// While ejection status and dynamic weight are not part of the EpochSetup event, we can supplement this information as follows:
	//   - Per convention, service events are delivered (asynchronously) in an *order-preserving* manner. Furthermore, weight changes or
	//     node ejection is entirely mediated by system smart contracts and delivered via service events.
	//   - Therefore, the EpochSetup event contains the up-to-date snapshot of the epoch participants. Any weight changes or node ejection
	//     that happened before should be reflected in the EpochSetup event. Specifically, the initial weight should be reduced and ejected
	//     nodes should be no longer listed in the EpochSetup event.
	//   - Hence, the following invariant must be satisfied by the system smart contracts for all active nodes in the upcoming epoch:
	//      (i) The Ejected flag is false. Node X being ejected in epoch N (necessarily via a service event emitted by the system
	//          smart contracts earlier) but also being listed in the setup event for the subsequent epoch (service event emitted by
	//          the system smart contracts later) is illegal.
	//     (ii) When the EpochSetup event is emitted / processed, the weight of all active nodes equals their InitialWeight and

	// For collector clusters, we rely on invariants (I) and (II) holding. See `committees.Cluster` for details, specifically function
	// `constructInitialClusterIdentities(..)`. While the system smart contract must satisfy this invariant, we run a sanity check below.
	activeIdentitiesLookup := u.parentState.CurrentEpoch.ActiveIdentities.Lookup() // lookup NodeID → DynamicIdentityEntry for nodes _active_ in the current epoch
	nextEpochActiveIdentities := make(flow.DynamicIdentityEntryList, 0, len(epochSetup.Participants))
	prevNodeID := epochSetup.Participants[0].NodeID
	for idx, nextEpochIdentitySkeleton := range epochSetup.Participants {
		// sanity checking invariant (I):
		if idx > 0 && !order.IdentifierCanonical(prevNodeID, nextEpochIdentitySkeleton.NodeID) {
			return protocol.NewInvalidServiceEventErrorf("epoch setup event lists active participants not in canonical ordering")
		}
		prevNodeID = nextEpochIdentitySkeleton.NodeID

		// sanity checking invariant (II.i):
		currentEpochDynamicProperties, found := activeIdentitiesLookup[nextEpochIdentitySkeleton.NodeID]
		if found && currentEpochDynamicProperties.Dynamic.Ejected { // invariance violated
			return protocol.NewInvalidServiceEventErrorf("node %v is ejected in current epoch %d but readmitted by EpochSetup event for epoch %d", nextEpochIdentitySkeleton.NodeID, u.parentState.CurrentEpochSetup.Counter, epochSetup.Counter)
		}

		nextEpochActiveIdentities = append(nextEpochActiveIdentities, &flow.DynamicIdentityEntry{
			NodeID: nextEpochIdentitySkeleton.NodeID,
			Dynamic: flow.DynamicIdentity{
				Weight:  nextEpochIdentitySkeleton.InitialWeight, // according to invariant (II.ii)
				Ejected: false,
			},
		})
	}

	// construct data container specifying next epoch
	u.state.NextEpoch = &flow.EpochStateContainer{
		SetupID:          epochSetup.ID(),
		CommitID:         flow.ZeroID,
		ActiveIdentities: nextEpochActiveIdentities,
	}

	// subsequent epoch commit event and update identities afterwards.
	u.nextEpochIdentitiesLookup = u.state.NextEpoch.ActiveIdentities.Lookup()

	return nil
}

// ProcessEpochCommit updates current protocol state with data from epoch commit event.
// Observing an epoch setup commit, transitions protocol state from setup to commit phase, at this point we have
// finished construction of the next epoch.
// As a result of this operation protocol state for next epoch will be committed.
// Expected errors during normal operations:
// - `protocol.InvalidServiceEventError` if the service event is invalid or is not a valid state transition for the current protocol state
func (u *stateMachine) ProcessEpochCommit(epochCommit *flow.EpochCommit) error {
	if u.state.InvalidStateTransitionAttempted {
		return nil // won't process new events if we are going to enter epoch fallback mode
	}
	if u.state.NextEpoch == nil {
		return protocol.NewInvalidServiceEventErrorf("protocol state has been setup yet")
	}
	if u.state.NextEpoch.CommitID != flow.ZeroID {
		return protocol.NewInvalidServiceEventErrorf("protocol state has already a commit event")
	}
	err := protocol.IsValidExtendingEpochCommit(
		epochCommit,
		u.parentState.NextEpochSetup,
		u.parentState.CurrentEpochSetup,
		u.parentState.EpochStatus())
	if err != nil {
		return fmt.Errorf("invalid epoch commit event: %w", err)
	}

	u.state.NextEpoch.CommitID = epochCommit.ID()
	return nil
}

// UpdateIdentity updates identity table with new identity entry.
// Should pass identity which is already present in the table, otherwise an exception will be raised.
// No errors are expected during normal operations.
func (u *stateMachine) UpdateIdentity(updated *flow.DynamicIdentityEntry) error {
	u.ensureLookupPopulated()
	prevEpochIdentity, foundInPrev := u.prevEpochIdentitiesLookup[updated.NodeID]
	if foundInPrev {
		prevEpochIdentity.Dynamic = updated.Dynamic
	}
	currentEpochIdentity, foundInCurrent := u.currentEpochIdentitiesLookup[updated.NodeID]
	if foundInCurrent {
		currentEpochIdentity.Dynamic = updated.Dynamic
	}
	nextEpochIdentity, foundInNext := u.nextEpochIdentitiesLookup[updated.NodeID]
	if foundInNext {
		nextEpochIdentity.Dynamic = updated.Dynamic
	}
	if !foundInPrev && !foundInCurrent && !foundInNext {
		return fmt.Errorf("expected to find identity for prev, current or next epoch, but (%v) was not found", updated.NodeID)
	}
	return nil
}

// SetInvalidStateTransitionAttempted sets a flag indicating that invalid state transition was attempted.
func (u *stateMachine) SetInvalidStateTransitionAttempted() {
	u.state.InvalidStateTransitionAttempted = true
}

// TransitionToNextEpoch updates the notion of 'current epoch', 'previous' and 'next epoch' in the protocol
// state. An epoch transition is only allowed when:
// - next epoch has been set up,
// - next epoch has been committed,
// - invalid state transition has not been attempted,
// - candidate block is in the next epoch.
// No errors are expected during normal operations.
func (u *stateMachine) TransitionToNextEpoch() error {
	if u.state.InvalidStateTransitionAttempted {
		return fmt.Errorf("invalid state transition has been attempted, no transition is allowed")
	}
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
		InvalidStateTransitionAttempted: false,
	}
	u.rebuildIdentityLookup()
	return nil
}

// View returns the view that is associated with this state updater.
// The view of the StateUpdater equals the view of the block carrying the respective updates.
func (u *stateMachine) View() uint64 {
	return u.view
}

// ParentState returns parent protocol state that is associated with this state updater.
func (u *stateMachine) ParentState() *flow.RichProtocolStateEntry {
	return u.parentState
}

// ensureLookupPopulated ensures that current and next epoch identities lookups are populated.
// We use this to avoid populating lookups on every UpdateIdentity call.
func (u *stateMachine) ensureLookupPopulated() {
	if len(u.currentEpochIdentitiesLookup) > 0 {
		return
	}
	u.rebuildIdentityLookup()
}

// rebuildIdentityLookup re-generates lookups of *active* participants for
// previous (optional, if u.state.PreviousEpoch ≠ nil), current (required) and
// next epoch (optional, if u.state.NextEpoch ≠ nil).
func (u *stateMachine) rebuildIdentityLookup() {
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
