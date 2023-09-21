package protocol_state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/state/protocol"
)

// Updater is a dedicated structure that encapsulates all logic for updating protocol state.
// Only protocol state updater knows how to update protocol state in a way that is consistent with the protocol.
// Protocol state updater implements the following state changes:
// - epoch setup: transitions current epoch from staking to setup phase, creates next epoch protocol state when processed.
// - epoch commit: transitions current epoch from setup to commit phase, commits next epoch protocol state when processed.
// - identity changes: updates identity table for current and next epoch(if available).
// - setting an invalid state transition flag: sets an invalid state transition flag for current epoch and next epoch(if available).
// All updates are applied to a copy of parent protocol state, so parent protocol state is not modified.
// It is NOT safe to use in concurrent environment.
type Updater struct {
	parentState *flow.RichProtocolStateEntry
	state       *flow.ProtocolStateEntry
	candidate   *flow.Header

	// nextEpochIdentitiesLookup is a map from NodeID → DynamicIdentityEntry for the _current_ epoch, containing the
	// same identities as in the EpochStateContainer `state.CurrentEpoch.Identities`. Note that map values are pointers,
	// so writes to map values will modify the respective DynamicIdentityEntry in EpochStateContainer.
	currentEpochIdentitiesLookup map[flow.Identifier]*flow.DynamicIdentityEntry

	// nextEpochIdentitiesLookup is a map from NodeID → DynamicIdentityEntry for the _next_ epoch, containing the
	// same identities as in the EpochStateContainer `state.NextEpoch.Identities`. Note that map values are pointers,
	// so writes to map values will modify the respective DynamicIdentityEntry in EpochStateContainer.
	nextEpochIdentitiesLookup map[flow.Identifier]*flow.DynamicIdentityEntry
}

var _ protocol.StateUpdater = (*Updater)(nil)

// NewUpdater creates a new protocol state updater.
func NewUpdater(candidate *flow.Header, parentState *flow.RichProtocolStateEntry) *Updater {
	updater := &Updater{
		parentState: parentState,
		state:       parentState.ProtocolStateEntry.Copy(),
		candidate:   candidate,
	}
	return updater
}

// Build returns updated protocol state entry, state ID and a flag indicating if there were any changes.
func (u *Updater) Build() (updatedState *flow.ProtocolStateEntry, stateID flow.Identifier, hasChanges bool) {
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
// No errors are expected during normal operations.
func (u *Updater) ProcessEpochSetup(epochSetup *flow.EpochSetup) error {
	if epochSetup.Counter != u.parentState.CurrentEpochSetup.Counter+1 {
		return fmt.Errorf("invalid epoch setup counter, expecting %v got %v", u.parentState.CurrentEpochSetup.Counter+1, epochSetup.Counter)
	}
	if u.state.NextEpoch != nil {
		return fmt.Errorf("protocol state has already a setup event")
	}
	if u.state.InvalidStateTransitionAttempted {
		return nil // won't process new events if we are in EECC
	}
	// Observing epoch setup impacts current and next epoch.
	// - For the current epoch, we stop returning identities from previous epoch.
	// Instead, we will return identities of current epoch + identities from the next epoch with 0 weight.
	// - For the next epoch we need to additionally include identities from previous(current) epoch with 0 weight.
	// We will do this in next steps:
	// 1. Add identities from current epoch setup event to currentEpochIdentities.
	// 2. Add identities from next epoch setup event to currentEpochIdentities, with 0 weight,
	//     but only if they are not present in currentEpochIdentities.
	// 3. Add identities from next epoch setup event to nextEpochIdentities.
	// 4. Add identities from current epoch setup event to nextEpochIdentities, with 0 weight,
	//    but only if they are not present in nextEpochIdentities.

	// lookup of dynamic data for current protocol state identities
	// by definition, this will include identities from current epoch + identities from previous epoch with 0 weight.
	identitiesStateLookup := u.parentState.CurrentEpoch.ActiveIdentities.Lookup()

	currentEpochSetupParticipants := u.parentState.CurrentEpochSetup.Participants
	// construct identities for current epoch: current epoch participants + next epoch participants with 0 weight
	currentEpochIdentities := make(flow.DynamicIdentityEntryList, 0, len(currentEpochSetupParticipants))
	// In this loop, we will perform step 1 from above.
	for _, identity := range currentEpochSetupParticipants {
		identityParentState := identitiesStateLookup[identity.NodeID]
		currentEpochIdentities = append(currentEpochIdentities, &flow.DynamicIdentityEntry{
			NodeID:  identity.NodeID,
			Dynamic: identityParentState.Dynamic,
		})
	}

	nextEpochIdentities := make(flow.DynamicIdentityEntryList, 0, len(currentEpochIdentities))
	currentEpochIdentitiesLookup := currentEpochIdentities.Lookup()
	// For an `identity` participating in the upcoming epoch, we effectively perform steps 2 and 3 from above within a single loop.
	for _, identity := range epochSetup.Participants {
		// Step 2: node is _not_ participating in the current epoch, but joining in the upcoming epoch.
		// The node is allowed to join the network already in this epoch's Setup Phase, but has weight 0.
		if _, found := currentEpochIdentitiesLookup[identity.NodeID]; !found {
			currentEpochIdentities = append(currentEpochIdentities, &flow.DynamicIdentityEntry{
				NodeID: identity.NodeID,
				Dynamic: flow.DynamicIdentity{
					Weight:  0,
					Ejected: identity.Ejected,
				},
			})
		}

		// Step 3: for the next epoch we include every identity from its setup event;
		// we give authority to epoch smart contract to decide who should be included in the next epoch and with what flags.
		nextEpochIdentities = append(nextEpochIdentities, &flow.DynamicIdentityEntry{
			NodeID: identity.NodeID,
			Dynamic: flow.DynamicIdentity{
				Weight:  identity.InitialWeight,
				Ejected: identity.Ejected,
			},
		})
	}

	nextEpochIdentitiesLookup := nextEpochIdentities.Lookup()
	// Step 4: we need to extend the next epoch's identities by adding identities that are leaving at the end of
	// the current epoch. Specifically, each identity from the current epoch that is _not_ listed in the
	// Setup Event for the next epoch is added with 0 weight and the _current_ value of the Ejected flag.
	for _, identity := range currentEpochSetupParticipants {
		if _, found := nextEpochIdentitiesLookup[identity.NodeID]; !found {
			identityParentState := identitiesStateLookup[identity.NodeID]
			nextEpochIdentities = append(nextEpochIdentities, &flow.DynamicIdentityEntry{
				NodeID: identity.NodeID,
				Dynamic: flow.DynamicIdentity{
					Weight:  0,
					Ejected: identityParentState.Dynamic.Ejected,
				},
			})
		}
	}

	// IMPORTANT: per convention, identities must be listed on canonical order!
	u.state.CurrentEpoch.ActiveIdentities = currentEpochIdentities.Sort(order.IdentifierCanonical)

	// construct protocol state entry for next epoch
	u.state.NextEpoch = &flow.EpochStateContainer{
		SetupID:          epochSetup.ID(),
		CommitID:         flow.ZeroID,
		ActiveIdentities: nextEpochIdentities.Sort(order.IdentifierCanonical),
	}

	// since identities have changed, rebuild identity lookups, so we can safely process
	// subsequent epoch commit event and update identities afterward.
	u.rebuildIdentityLookup()

	return nil
}

// ProcessEpochCommit updates current protocol state with data from epoch commit event.
// Observing an epoch setup commit, transitions protocol state from setup to commit phase, at this point we have
// finished construction of the next epoch.
// As a result of this operation protocol state for next epoch will be committed.
// No errors are expected during normal operations.
func (u *Updater) ProcessEpochCommit(epochCommit *flow.EpochCommit) error {
	if epochCommit.Counter != u.parentState.CurrentEpochSetup.Counter+1 {
		return fmt.Errorf("invalid epoch commit counter, expecting %v got %v", u.parentState.CurrentEpochSetup.Counter+1, epochCommit.Counter)
	}
	if u.state.NextEpoch == nil {
		return fmt.Errorf("protocol state has been setup yet")
	}
	if u.state.NextEpoch.CommitID != flow.ZeroID {
		return fmt.Errorf("protocol state has already a commit event")
	}
	if u.state.InvalidStateTransitionAttempted {
		return nil // won't process new events if we are going to enter EECC
	}

	u.state.NextEpoch.CommitID = epochCommit.ID()
	return nil
}

// UpdateIdentity updates identity table with new identity entry.
// Should pass identity which is already present in the table, otherwise an exception will be raised.
// No errors are expected during normal operations.
func (u *Updater) UpdateIdentity(updated *flow.DynamicIdentityEntry) error {
	u.ensureLookupPopulated()
	currentEpochIdentity, found := u.currentEpochIdentitiesLookup[updated.NodeID]
	if !found {
		return fmt.Errorf("expected to find identity for current epoch, but (%v) not found", updated.NodeID)
	}

	currentEpochIdentity.Dynamic = updated.Dynamic
	if u.state.NextEpoch != nil {
		nextEpochIdentity, found := u.nextEpochIdentitiesLookup[updated.NodeID]
		if found {
			nextEpochIdentity.Dynamic = updated.Dynamic
		}
	}
	return nil
}

// SetInvalidStateTransitionAttempted sets a flag indicating that invalid state transition was attempted.
func (u *Updater) SetInvalidStateTransitionAttempted() {
	u.state.InvalidStateTransitionAttempted = true
}

// TransitionToNextEpoch updates the notion of 'current epoch', 'previous' and 'next epoch' in the protocol
// state. An epoch transition is only allowed when:
// - next epoch has been set up,
// - next epoch has been committed,
// - invalid state transition has not been attempted,
// - candidate block is in the next epoch.
// No errors are expected during normal operations.
func (u *Updater) TransitionToNextEpoch() error {
	nextEpoch := u.state.NextEpoch
	// Check if there is next epoch protocol state
	if nextEpoch == nil {
		return fmt.Errorf("protocol state has not been setup yet")
	}

	// Check if there is a commit event for next epoch
	if nextEpoch.CommitID == flow.ZeroID {
		return fmt.Errorf("protocol state has not been committed yet")
	}
	if u.state.InvalidStateTransitionAttempted {
		return fmt.Errorf("invalid state transition has been attempted, no transition is allowed")
	}
	// Check if we are at the next epoch, only then a transition is allowed
	if u.candidate.View < u.parentState.NextEpochSetup.FirstView {
		return fmt.Errorf("protocol state transition is only allowed when enterring next epoch")
	}
	u.state = &flow.ProtocolStateEntry{
		PreviousEpochEventIDs:           u.state.CurrentEpoch.EventIDs(),
		CurrentEpoch:                    *u.state.NextEpoch,
		InvalidStateTransitionAttempted: false,
	}
	u.rebuildIdentityLookup()
	return nil
}

// Block returns the block header that is associated with this state updater.
// StateUpdater is created for a specific block where protocol state changes are incorporated.
func (u *Updater) Block() *flow.Header {
	return u.candidate
}

// ParentState returns parent protocol state that is associated with this state updater.
func (u *Updater) ParentState() *flow.RichProtocolStateEntry {
	return u.parentState
}

// ensureLookupPopulated ensures that current and next epoch identities lookups are populated.
// We use this to avoid populating lookups on every UpdateIdentity call.
func (u *Updater) ensureLookupPopulated() {
	if len(u.currentEpochIdentitiesLookup) > 0 {
		return
	}
	u.rebuildIdentityLookup()
}

// rebuildIdentityLookup re-generates `currentEpochIdentitiesLookup` and `nextEpochIdentitiesLookup` from the
// underlying identity lists `state.CurrentEpoch.Identities` and `state.NextEpoch.Identities`, respectively.
func (u *Updater) rebuildIdentityLookup() {
	u.currentEpochIdentitiesLookup = u.state.CurrentEpoch.ActiveIdentities.Lookup()
	if u.state.NextEpoch != nil {
		u.nextEpochIdentitiesLookup = u.state.NextEpoch.ActiveIdentities.Lookup()
	} else {
		u.nextEpochIdentitiesLookup = nil
	}
}
