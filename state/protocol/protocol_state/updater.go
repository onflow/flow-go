package protocol_state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/state/protocol"
)

// Updater is a dedicated structure that encapsulates all logic for updating protocol state.
// Only protocol state updater knows how to update protocol state in a way that is consistent with the protocol.
// Protocol state updater supports processing next types of changes:
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

	currentEpochIdentitiesLookup map[flow.Identifier]*flow.DynamicIdentityEntry
	nextEpochIdentitiesLookup    map[flow.Identifier]*flow.DynamicIdentityEntry
}

var _ protocol.StateUpdater = (*Updater)(nil)

// newUpdater creates a new protocol state updater, when candidate block enters new epoch we will discard
// previous protocol state and move to the next epoch protocol state.
func newUpdater(candidate *flow.Header, parentState *flow.RichProtocolStateEntry) *Updater {
	updater := &Updater{
		parentState: parentState,
		state:       parentState.ProtocolStateEntry.Copy(),
		candidate:   candidate,
	}

	// Check if we are at the first block of new epoch.
	// This check will be true only if parent block is in previous epoch and candidate block is a new epoch.
	if candidate.View > parentState.CurrentEpochSetup.FinalView {
		// discard protocol state from the previous epoch and update the current protocol state
		// identities and other changes are applied to both current and next epochs so this object is up-to-date
		// with all applied deltas.
		updater.state = updater.state.NextEpochProtocolState
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

// ProcessEpochSetup updates current protocol state with data from epoch setup event.
// Processing epoch setup event also affects identity table for current epoch.
// Observing an epoch setup event, transitions protocol state from staking to setup phase, we stop returning
// identities from previous+current epochs and start returning identities from current+next epochs.
// As a result of this operation protocol state for the next epoch will be created.
// No errors are expected during normal operations.
func (u *Updater) ProcessEpochSetup(epochSetup *flow.EpochSetup) error {
	if epochSetup.Counter != u.parentState.CurrentEpochSetup.Counter+1 {
		return fmt.Errorf("invalid epoch setup counter, expecting %v got %v", u.parentState.CurrentEpochSetup.Counter+1, epochSetup.Counter)
	}
	if u.state.NextEpochProtocolState != nil {
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
	// but only if they are not present in currentEpochIdentities.
	// 3. Add identities from next epoch setup event to nextEpochIdentities.
	// 4. Add identities from current epoch setup event to nextEpochIdentities, with 0 weight,
	// but only if they are not present in nextEpochIdentities.

	// lookup of dynamic data for current protocol state identities
	// by definition, this will include identities from current epoch + identities from previous epoch with 0 weight.
	identitiesStateLookup := u.parentState.ProtocolStateEntry.Identities.Lookup()

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
	// in this loop, we will fill participants for both current and next epochs, effectively performing steps 2 and 3 from above.
	for _, identity := range epochSetup.Participants {
		// if present in current epoch, skip
		if _, found := currentEpochIdentitiesLookup[identity.NodeID]; !found {
			currentEpochIdentities = append(currentEpochIdentities, &flow.DynamicIdentityEntry{
				NodeID: identity.NodeID,
				Dynamic: flow.DynamicIdentity{
					Weight:  0,
					Ejected: identity.Ejected,
				},
			})
		}

		// for the next epoch we include identities from setup event,
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
	// Finally, we need to augment next epoch identities with identities from current epoch with 0 weight,
	// effectively performing step 4 from above.
	// For the next epoch, we include identities(with 0 weight) from setup event but take the ejected flag from parent protocol state.
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

	u.state.Identities = currentEpochIdentities.Sort(order.IdentifierCanonical)

	// construct protocol state entry for next epoch
	u.state.NextEpochProtocolState = &flow.ProtocolStateEntry{
		CurrentEpochEventIDs: flow.EventIDs{
			SetupID:  epochSetup.ID(),
			CommitID: flow.ZeroID,
		},
		PreviousEpochEventIDs:           u.state.CurrentEpochEventIDs,
		Identities:                      nextEpochIdentities.Sort(order.IdentifierCanonical),
		InvalidStateTransitionAttempted: false,
		NextEpochProtocolState:          nil,
	}

	// since identities have changed, invalidate lookup, so we can safely process epoch setup
	// and update identities afterward.
	u.invalidateLookup()

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
	if u.state.NextEpochProtocolState == nil {
		return fmt.Errorf("protocol state has been setup yet")
	}
	if u.state.NextEpochProtocolState.CurrentEpochEventIDs.CommitID != flow.ZeroID {
		return fmt.Errorf("protocol state has already a commit event")
	}
	if u.state.InvalidStateTransitionAttempted {
		return nil // won't process new events if we are going to enter EECC
	}

	u.state.NextEpochProtocolState.CurrentEpochEventIDs.CommitID = epochCommit.ID()
	return nil
}

// UpdateIdentity updates identity table with new identity entry.
// Should pass identity which is already present in the table, otherwise an exception will be raised.
// No errors are expected during normal operations.
func (u *Updater) UpdateIdentity(updated *flow.DynamicIdentityEntry) error {
	u.ensureLookupPopulated()

	newData := updated.Dynamic

	currentEpochIdentity, found := u.currentEpochIdentitiesLookup[updated.NodeID]
	if !found {
		return fmt.Errorf("expected to find identity for current epoch, but (%v) not found", updated.NodeID)
	}
	currentEpochIdentity.Dynamic = newData

	if u.state.NextEpochProtocolState != nil {
		nextEpochIdentity, found := u.nextEpochIdentitiesLookup[updated.NodeID]
		if found {
			nextEpochIdentity.Dynamic = newData
		}
	}
	return nil
}

// SetInvalidStateTransitionAttempted sets a flag indicating that invalid state transition was attempted.
func (u *Updater) SetInvalidStateTransitionAttempted() {
	u.state.InvalidStateTransitionAttempted = true
	if u.state.NextEpochProtocolState != nil {
		u.state.NextEpochProtocolState.InvalidStateTransitionAttempted = true
	}
}

// Block returns the block header that is associated with this state updater.
// StateUpdater is created for a specific block where protocol state changes are incorporated.
func (u *Updater) Block() *flow.Header {
	return u.candidate
}

// ensureLookupPopulated ensures that current and next epoch identities lookups are populated.
// We use this to avoid populating lookups on every UpdateIdentity call.
func (u *Updater) ensureLookupPopulated() {
	if len(u.currentEpochIdentitiesLookup) > 0 {
		return
	}
	u.invalidateLookup()
}

// invalidateLookup invalidates current and next epoch identities lookup.
func (u *Updater) invalidateLookup() {
	u.currentEpochIdentitiesLookup = u.state.Identities.Lookup()
	if u.state.NextEpochProtocolState != nil {
		u.nextEpochIdentitiesLookup = u.state.NextEpochProtocolState.Identities.Lookup()
	}
}
