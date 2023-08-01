package protocol_state

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
)

type Updater struct {
	parentState *flow.RichProtocolStateEntry
	state       *flow.ProtocolStateEntry

	currentEpochIdentitiesLookup map[flow.Identifier]*flow.DynamicIdentityEntry
	nextEpochIdentitiesLookup    map[flow.Identifier]*flow.DynamicIdentityEntry
}

func newUpdater(candidate *flow.Header, parentState *flow.RichProtocolStateEntry) *Updater {
	updater := &Updater{
		parentState: parentState,
		state:       parentState.ProtocolStateEntry.Copy(),
	}

	// check if we are at first block of new epoch. This check will be true only if parent block is in
	// previous epoch and candidate block is new epoch.
	if candidate.View > parentState.CurrentEpochSetup.FinalView {
		// discard protocol state from previous epoch and update the current protocol state
		// identities and other changes are applied to both current and next epochs so this object is up-to-date
		// with all applied deltas.
		updater.state = updater.state.NextEpochProtocolState
	}

	return updater
}

func (u *Updater) Build() (updatedState *flow.ProtocolStateEntry, stateID flow.Identifier, hasChanges bool) {
	updatedState = u.state.Copy()
	stateID = updatedState.ID()
	hasChanges = stateID != u.parentState.ID()
	return
}

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
	// - For current epoch we stop returning identities from previous epoch.
	// Instead, we will return identities of current epoch + identities from next epoch with 0 weight.
	// - For next epoch we need to additionally include identities from previous(current) epoch with 0 weight.
	// We will do this in next steps:
	// 1. Add identities from current epoch setup event to currentEpochIdentities.
	// 2. Add identities from next epoch setup event to currentEpochIdentities, with 0 weight,
	// but only if they are not present in currentEpochIdentities.
	// 3. Add identities from next epoch setup event to nextEpochIdentities.
	// 4. Add identities from current epoch setup event to nextEpochIdentities, with 0 weight,
	// but only if they are not present in nextEpochIdentities.

	identitiesStateLookup := u.parentState.Identities.Lookup()

	// construct identities for current epoch: current epoch participants + next epoch participants with 0 weight
	var currentEpochIdentities flow.DynamicIdentityEntryList
	// In this loop we will perform step 1 from above.
	for _, identity := range u.parentState.CurrentEpochSetup.Participants {
		identityParentState := identitiesStateLookup[identity.NodeID]
		currentEpochIdentities = append(currentEpochIdentities, &flow.DynamicIdentityEntry{
			NodeID:  identity.NodeID,
			Dynamic: identityParentState.DynamicIdentity,
		})
	}

	var nextEpochIdentities flow.DynamicIdentityEntryList
	currentEpochIdentitiesLookup := currentEpochIdentities.Lookup()
	// in this loop we will fill participants for both current and next epochs, effectively performing steps 2 and 3 from above.
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

		// for next epoch we include identities from setup event,
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
	// finally we need to augment next epoch identities with identities from current epoch with 0 weight, effectively performing step 4 from above.
	// for next epoch we include identities(with 0 weight) from setup event but take ejected flag from parent protocol state.
	for _, identity := range u.parentState.CurrentEpochSetup.Participants {
		if _, found := nextEpochIdentitiesLookup[identity.NodeID]; !found {
			identityParentState := identitiesStateLookup[identity.NodeID]
			nextEpochIdentities = append(nextEpochIdentities, &flow.DynamicIdentityEntry{
				NodeID: identity.NodeID,
				Dynamic: flow.DynamicIdentity{
					Weight:  0,
					Ejected: identityParentState.Ejected,
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

	return nil
}

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
		if !found {
			return fmt.Errorf("expected to find identity for next epoch, but (%v) not found", updated.NodeID)
		}
		nextEpochIdentity.Dynamic = newData
	}
	return nil
}

func (u *Updater) SetInvalidStateTransitionAttempted() {
	u.state.InvalidStateTransitionAttempted = true
	if u.state.NextEpochProtocolState != nil {
		u.state.NextEpochProtocolState.InvalidStateTransitionAttempted = true
	}
}

func (u *Updater) ensureLookupPopulated() {
	if len(u.currentEpochIdentitiesLookup) > 0 {
		return
	}

	u.currentEpochIdentitiesLookup = u.state.Identities.Lookup()
	if u.state.NextEpochProtocolState != nil {
		u.nextEpochIdentitiesLookup = u.state.NextEpochProtocolState.Identities.Lookup()
	}
}
