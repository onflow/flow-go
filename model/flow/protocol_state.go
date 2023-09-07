package flow

import (
	"fmt"
	"sort"
)

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

	// ensure data is consistent
	if protocolState.PreviousEpochEventIDs.SetupID != ZeroID {
		if protocolState.PreviousEpochEventIDs.SetupID != previousEpochSetup.ID() {
			return nil, fmt.Errorf("supplied previous epoch setup (%x) does not match protocol state (%x)",
				previousEpochSetup.ID(),
				protocolState.PreviousEpochEventIDs.SetupID)
		}
		if protocolState.PreviousEpochEventIDs.CommitID != previousEpochCommit.ID() {
			return nil, fmt.Errorf("supplied previous epoch commit (%x) does not match protocol state (%x)",
				previousEpochCommit.ID(),
				protocolState.PreviousEpochEventIDs.CommitID)
		}
	}
	if protocolState.CurrentEpochEventIDs.SetupID != currentEpochSetup.ID() {
		return nil, fmt.Errorf("supplied current epoch setup (%x) does not match protocol state (%x)",
			currentEpochSetup.ID(),
			protocolState.CurrentEpochEventIDs.SetupID)
	}
	if protocolState.CurrentEpochEventIDs.CommitID != currentEpochCommit.ID() {
		return nil, fmt.Errorf("supplied current epoch commit (%x) does not match protocol state (%x)",
			currentEpochCommit.ID(),
			protocolState.CurrentEpochEventIDs.CommitID)
	}

	var err error
	nextEpochProtocolState := protocolState.NextEpochProtocolState
	// if next epoch has been already committed, fill in data for it as well.
	if nextEpochProtocolState != nil {
		// sanity check consistency of input data
		if nextEpochProtocolState.CurrentEpochEventIDs.SetupID != nextEpochSetup.ID() {
			return nil, fmt.Errorf("inconsistent EpochSetup for constucting RichProtocolStateEntry, next protocol state states ID %v while input event has ID %v",
				protocolState.NextEpochProtocolState.CurrentEpochEventIDs.SetupID, nextEpochSetup.ID())
		}
		if nextEpochProtocolState.CurrentEpochEventIDs.CommitID != nextEpochCommit.ID() {
			return nil, fmt.Errorf("inconsistent EpochCommit for constucting RichProtocolStateEntry, next protocol state states ID %v while input event has ID %v",
				protocolState.NextEpochProtocolState.CurrentEpochEventIDs.CommitID, nextEpochCommit.ID())
		}

		// sanity check consistency of input data
		if nextEpochProtocolState.CurrentEpochEventIDs.SetupID != nextEpochSetup.ID() {
			return nil, fmt.Errorf("inconsistent EpochSetup for constucting RichProtocolStateEntry, next protocol state states ID %v while input event has ID %v",
				protocolState.NextEpochProtocolState.CurrentEpochEventIDs.SetupID, nextEpochSetup.ID())
		}
		if nextEpochProtocolState.CurrentEpochEventIDs.CommitID != nextEpochCommit.ID() {
			return nil, fmt.Errorf("inconsistent EpochCommit for constucting RichProtocolStateEntry, next protocol state states ID %v while input event has ID %v",
				protocolState.NextEpochProtocolState.CurrentEpochEventIDs.CommitID, nextEpochCommit.ID())
		}

		// if next epoch is available, it means that we have observed epoch setup event and we are not anymore in staking phase,
		// so we need to build the identity table using current and next epoch setup events.
		// so we need to build the identity table using current and next epoch setup events.
		result.Identities, err = buildIdentityTable(
			protocolState.Identities,
			currentEpochSetup.Participants,
			nextEpochSetup.Participants,
		)
		if err != nil {
			return nil, fmt.Errorf("could not build identity table for setup/commit phase: %w", err)
		}

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

// Copy returns a full copy of the entry.
func (e *ProtocolStateEntry) Copy() *ProtocolStateEntry {
	if e == nil {
		return nil
	}
	return &ProtocolStateEntry{
		CurrentEpochEventIDs:            e.CurrentEpochEventIDs,
		PreviousEpochEventIDs:           e.PreviousEpochEventIDs,
		Identities:                      e.Identities.Copy(),
		InvalidStateTransitionAttempted: e.InvalidStateTransitionAttempted,
		NextEpochProtocolState:          e.NextEpochProtocolState.Copy(),
	}
}

// Copy returns a full copy of rich protocol state entry.
// Embedded service events are copied by reference (not deep-copied).
func (e *RichProtocolStateEntry) Copy() *RichProtocolStateEntry {
	if e == nil {
		return nil
	}
	return &RichProtocolStateEntry{
		ProtocolStateEntry:     e.ProtocolStateEntry.Copy(),
		CurrentEpochSetup:      e.CurrentEpochSetup,
		CurrentEpochCommit:     e.CurrentEpochCommit,
		PreviousEpochSetup:     e.PreviousEpochSetup,
		PreviousEpochCommit:    e.PreviousEpochCommit,
		Identities:             e.Identities.Copy(),
		NextEpochProtocolState: e.NextEpochProtocolState.Copy(),
	}
}

// EpochStatus returns epoch status for the current protocol state.
func (e *ProtocolStateEntry) EpochStatus() *EpochStatus {
	var nextEpoch EventIDs
	if e.NextEpochProtocolState != nil {
		nextEpoch = e.NextEpochProtocolState.CurrentEpochEventIDs
	}
	return &EpochStatus{
		PreviousEpoch:                   e.PreviousEpochEventIDs,
		CurrentEpoch:                    e.CurrentEpochEventIDs,
		NextEpoch:                       nextEpoch,
		InvalidServiceEventIncorporated: e.InvalidStateTransitionAttempted,
	}
}

func (ll DynamicIdentityEntryList) Lookup() map[Identifier]*DynamicIdentityEntry {
	result := make(map[Identifier]*DynamicIdentityEntry, len(ll))
	for _, entry := range ll {
		result[entry.NodeID] = entry
	}
	return result
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

// ByNodeID gets a node from the list by node ID.
func (ll DynamicIdentityEntryList) ByNodeID(nodeID Identifier) (*DynamicIdentityEntry, bool) {
	for _, identity := range ll {
		if identity.NodeID == nodeID {
			return identity, true
		}
	}
	return nil, false
}

func (ll DynamicIdentityEntryList) Copy() DynamicIdentityEntryList {
	dup := make(DynamicIdentityEntryList, 0, len(ll))

	lenList := len(ll)
	for i := 0; i < lenList; i++ {
		// copy the object
		next := *(ll[i])
		dup = append(dup, &next)
	}
	return dup
}

// Sort sorts the list by the input ordering. Returns a new, sorted list without modifying the input.
func (ll DynamicIdentityEntryList) Sort(less IdentifierOrder) DynamicIdentityEntryList {
	dup := ll.Copy()
	sort.Slice(dup, func(i int, j int) bool {
		return less(dup[i].NodeID, dup[j].NodeID)
	})
	return dup
}

// buildIdentityTable constructs the full identity table for the target epoch by combining data from:
//  1. The target epoch's Dynamic Identities.
//  2. The target epoch's IdentitySkeletons
//     (recorded in EpochSetup event and immutable throughout the epoch).
//  3. [optional] An adjacent epoch's IdentitySkeletons (can be empty or nil), as recorded in the
//     adjacent epoch's setup event. For a target epoch N, the epochs N-1 and N+1 are defined to be
//     adjacent. Adjacent epochs do not _necessarily_ exist (e.g. consider a spork comprising only
//     a single epoch), in which case this input is nil or empty.
//
// It also performs sanity checks to make sure that the data is consistent.
// No errors are expected during normal operation.
func buildIdentityTable(
	targetEpochDynamicIdentities DynamicIdentityEntryList,
	targetEpochIdentitySkeletons IdentityList, // TODO: change to `IdentitySkeletonList`
	adjacentEpochIdentitySkeletons IdentityList, // TODO: change to `IdentitySkeletonList`
) (IdentityList, error) {
	// produce a unique set for current and previous epoch participants
	allEpochParticipants := targetEpochIdentitySkeletons.Union(adjacentEpochIdentitySkeletons)
	// sanity check: size of identities should be equal to previous and current epoch participants combined
	if len(allEpochParticipants) != len(targetEpochDynamicIdentities) {
		return nil, fmt.Errorf("invalid number of identities in protocol state: expected %d, got %d", len(allEpochParticipants), len(targetEpochDynamicIdentities))
	}

	// build full identity table for current epoch
	var result IdentityList
	for i, identity := range targetEpochDynamicIdentities {
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
