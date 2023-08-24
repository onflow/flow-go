package flow

import "fmt"

// DynamicIdentityEntry encapsulates nodeID and dynamic portion of identity.
type DynamicIdentityEntry struct {
	NodeID  Identifier
	Dynamic DynamicIdentity
}

type DynamicIdentityEntryList []*DynamicIdentityEntry

// ProtocolStateEntry represents a snapshot of the identity table (i.e. the set of all notes authorized to
// be part of the network) at some point in time. It allows to reconstruct the state of identity table using
// epoch setup events and dynamic identities. It tracks attempts of invalid state transitions.
// It also holds information about the next epoch, if it has been already committed.
// This structure is used to persist protocol state in the database.
//
// Note that the current implementation does not store the identity table directly. Instead, we store
// the original events that constituted the _initial_ identity table at the beginning of the epoch
// plus some modifiers. We intend to restructure this code soon.
// TODO: https://github.com/onflow/flow-go/issues/4649
type ProtocolStateEntry struct {
	// setup and commit event IDs for current epoch.
	CurrentEpochEventIDs EventIDs
	// setup and commit event IDs for previous epoch.
	PreviousEpochEventIDs EventIDs
	// Part of identity table that can be changed during the epoch.
	// Always sorted in canonical order.
	Identities DynamicIdentityEntryList
	// InvalidStateTransitionAttempted encodes whether an invalid state transition
	// has been detected in this fork. When this happens, epoch fallback is triggered
	// AFTER the fork is finalized.
	InvalidStateTransitionAttempted bool
	// NextEpochProtocolState describes protocol state of the next epoch
	NextEpochProtocolState *ProtocolStateEntry
}

// RichProtocolStateEntry is a ProtocolStateEntry which has additional fields that are cached
// from storage layer for convenience.
// Using this structure instead of ProtocolStateEntry allows us to avoid querying
// the database for epoch setups and commits and full identity table.
// It holds several invariants, such as:
//   - CurrentEpochSetup and CurrentEpochCommit are for the same epoch. Never nil.
//   - PreviousEpochSetup and PreviousEpochCommit are for the same epoch. Can be nil.
//   - Identities is a full identity table for the current epoch.
//     Identities are sorted in canonical order. Without duplicates. Never nil.
//   - NextEpochProtocolState is a protocol state for the next epoch. Can be nil.
type RichProtocolStateEntry struct {
	*ProtocolStateEntry

	CurrentEpochSetup   *EpochSetup
	CurrentEpochCommit  *EpochCommit
	PreviousEpochSetup  *EpochSetup
	PreviousEpochCommit *EpochCommit
	Identities          IdentityList

	NextEpochProtocolState *RichProtocolStateEntry
}

// NewRichProtocolStateEntry constructs a rich protocol state entry from a protocol state entry and additional data.
// No errors are expected during normal operation.
func NewRichProtocolStateEntry(
	protocolState *ProtocolStateEntry,
	previousEpochSetup *EpochSetup,
	previousEpochCommit *EpochCommit,
	currentEpochSetup *EpochSetup,
	currentEpochCommit *EpochCommit,
	nextEpochSetup *EpochSetup,
	nextEpochCommit *EpochCommit,
) (*RichProtocolStateEntry, error) {
	result := &RichProtocolStateEntry{
		ProtocolStateEntry:     protocolState,
		CurrentEpochSetup:      currentEpochSetup,
		CurrentEpochCommit:     currentEpochCommit,
		PreviousEpochSetup:     previousEpochSetup,
		PreviousEpochCommit:    previousEpochCommit,
		Identities:             nil,
		NextEpochProtocolState: nil,
	}

	var err error
	// if next epoch has been already committed, fill in data for it as well.
	if protocolState.NextEpochProtocolState != nil {
		// sanity check consistency of input data
		if protocolState.NextEpochProtocolState.CurrentEpochEventIDs.SetupID != nextEpochSetup.ID() {
			return nil, fmt.Errorf("inconsistent EpochSetup for constucting RichProtocolStateEntry, next protocol state states ID %v while input event has ID %v",
				protocolState.NextEpochProtocolState.CurrentEpochEventIDs.SetupID, nextEpochSetup.ID())
		}
		if protocolState.NextEpochProtocolState.CurrentEpochEventIDs.CommitID != nextEpochCommit.ID() {
			return nil, fmt.Errorf("inconsistent EpochCommit for constucting RichProtocolStateEntry, next protocol state states ID %v while input event has ID %v",
				protocolState.NextEpochProtocolState.CurrentEpochEventIDs.CommitID, nextEpochCommit.ID())
		}

		// if next epoch is available, it means that we have observed epoch setup event and we are not anymore in staking phase,
		// so we need to build the identity table using current and next epoch setup events.
		result.Identities, err = buildIdentityTable(
			protocolState.Identities,
			currentEpochSetup.Participants,
			nextEpochSetup.Participants,
		)
		if err != nil {
			return nil, fmt.Errorf("could not build identity table for setup/commit phase: %w", err)
		}

		nextEpochProtocolState := protocolState.NextEpochProtocolState
		nextEpochIdentityTable, err := buildIdentityTable(
			nextEpochProtocolState.Identities,
			nextEpochSetup.Participants,
			currentEpochSetup.Participants,
		)
		if err != nil {
			return nil, fmt.Errorf("could not build next epoch identity table: %w", err)
		}

		// fill identities for next epoch
		result.NextEpochProtocolState = &RichProtocolStateEntry{
			ProtocolStateEntry:     nextEpochProtocolState,
			CurrentEpochSetup:      nextEpochSetup,
			CurrentEpochCommit:     nextEpochCommit,
			PreviousEpochSetup:     result.CurrentEpochSetup,  // previous epoch setup is current epoch setup
			PreviousEpochCommit:    result.CurrentEpochCommit, // previous epoch setup is current epoch setup
			Identities:             nextEpochIdentityTable,
			NextEpochProtocolState: nil, // always nil
		}
	} else {
		// if next epoch is not yet created, it means that we are in staking phase,
		// so we need to build the identity table using previous and current epoch setup events.
		var otherIdentities IdentityList
		if previousEpochSetup != nil {
			otherIdentities = previousEpochSetup.Participants
		}
		result.Identities, err = buildIdentityTable(
			protocolState.Identities,
			currentEpochSetup.Participants,
			otherIdentities,
		)
		if err != nil {
			return nil, fmt.Errorf("could not build identity table for staking phase: %w", err)
		}
	}

	return result, nil
}

// ID returns hash of entry by hashing all fields.
func (e *ProtocolStateEntry) ID() Identifier {
	if e == nil {
		return ZeroID
	}
	body := struct {
		CurrentEpochEventIDs            Identifier
		PreviousEpochEventIDs           Identifier
		Identities                      DynamicIdentityEntryList
		InvalidStateTransitionAttempted bool
		NextEpochProtocolStateID        Identifier
	}{
		CurrentEpochEventIDs:            e.CurrentEpochEventIDs.ID(),
		PreviousEpochEventIDs:           e.PreviousEpochEventIDs.ID(),
		Identities:                      e.Identities,
		InvalidStateTransitionAttempted: e.InvalidStateTransitionAttempted,
		NextEpochProtocolStateID:        e.NextEpochProtocolState.ID(),
	}
	return MakeID(body)
}

// Sorted returns whether the list is sorted by the input ordering.
func (ll DynamicIdentityEntryList) Sorted(less IdentifierOrder) bool {
	for i := 0; i < len(ll)-1; i++ {
		a := ll[i]
		b := ll[i+1]
		if !less(a.NodeID, b.NodeID) {
			return false
		}
	}
	return true
}

// buildIdentityTable builds identity table for current epoch combining data from:
//   - the current epoch's Dynamic Identities
//   - and previous + current EpochSetup events
//
// It also performs sanity checks to make sure that the data is consistent.
// No errors are expected during normal operation.
func buildIdentityTable(
	dynamicIdentities DynamicIdentityEntryList,
	coreIdentities, otherIdentities IdentityList,
) (IdentityList, error) {
	// produce a unique set for current and previous epoch participants
	allEpochParticipants := coreIdentities.Union(otherIdentities)
	// sanity check: size of identities should be equal to previous and current epoch participants combined
	// This is because
	if len(allEpochParticipants) != len(dynamicIdentities) {
		return nil, fmt.Errorf("invalid number of identities in protocol state: expected %d, got %d", len(allEpochParticipants), len(dynamicIdentities))
	}

	// build full identity table for current epoch
	var result IdentityList
	for i, identity := range dynamicIdentities {
		// sanity check: identities should be sorted in canonical order
		if identity.NodeID != allEpochParticipants[i].NodeID {
			return nil, fmt.Errorf("identites in protocol state are not in canonical order: expected %s, got %s", allEpochParticipants[i].NodeID, identity.NodeID)
		}
		result = append(result, &Identity{
			IdentitySkeleton: allEpochParticipants[i].IdentitySkeleton,
			DynamicIdentity:  identity.Dynamic,
		})
	}
	return result, nil
}

// DynamicIdentityEntryListFromIdentities converts IdentityList to DynamicIdentityEntryList.
func DynamicIdentityEntryListFromIdentities(identities IdentityList) DynamicIdentityEntryList {
	dynamicIdentities := make(DynamicIdentityEntryList, 0, len(identities))
	for _, identity := range identities {
		dynamicIdentities = append(dynamicIdentities, &DynamicIdentityEntry{
			NodeID:  identity.NodeID,
			Dynamic: identity.DynamicIdentity,
		})
	}
	return dynamicIdentities
}
