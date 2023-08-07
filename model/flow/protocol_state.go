package flow

import "sort"
import "fmt"

// DynamicIdentityEntry encapsulates nodeID and dynamic portion of identity.
type DynamicIdentityEntry struct {
	NodeID  Identifier
	Dynamic DynamicIdentity
}

type DynamicIdentityEntryList []*DynamicIdentityEntry

// ProtocolStateEntry holds information about the protocol state at some point in time.
// It allows to reconstruct the state of identity table using epoch setup events and dynamic identities.
// It tracks attempts of invalid state transitions.
// It also holds information about the next epoch, if it has been already committed.
// This structure is used to persist protocol state in the database.
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
//   - PreviousEpochSetup and PreviousEpochCommit are for the same epoch. Never nil.
//   - Identities is a full identity table for the current epoch.
//     Identities are sorted in canonical order. Without duplicates. Never nil.
//   - NextEpochProtocolState is a protocol state for the next epoch. Can be nil.
type RichProtocolStateEntry struct {
	ProtocolStateEntry

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
	protocolState ProtocolStateEntry,
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
	result.Identities, err = buildIdentityTable(protocolState.Identities, result.PreviousEpochSetup, result.CurrentEpochSetup)
	if err != nil {
		return nil, fmt.Errorf("could not build identity table: %w", err)
	}

	// if next epoch has been already committed, fill in data for it as well.
	if protocolState.NextEpochProtocolState != nil {
		nextEpochProtocolState := *protocolState.NextEpochProtocolState
		nextEpochIdentityTable, err := buildIdentityTable(nextEpochProtocolState.Identities, result.CurrentEpochSetup, nextEpochSetup)
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

// Sort sorts the list by the input ordering.
func (ll DynamicIdentityEntryList) Sort(less IdentifierOrder) DynamicIdentityEntryList {
	dup := ll.Copy()
	sort.Slice(dup, func(i int, j int) bool {
		return less(dup[i].NodeID, dup[j].NodeID)
	})
	return dup
}

// buildIdentityTable builds identity table for current epoch combining data from previous, current epoch setups and dynamic identities
// that are stored in protocol state. It also performs sanity checks to make sure that data is consistent.
// No errors are expected during normal operation.
func buildIdentityTable(
	dynamicIdentities DynamicIdentityEntryList,
	previousEpochSetup, currentEpochSetup *EpochSetup,
) (IdentityList, error) {
	var previousEpochParticipants IdentityList
	if previousEpochSetup != nil {
		previousEpochParticipants = previousEpochSetup.Participants
	}
	// produce a unique set for current and previous epoch participants
	allEpochParticipants := currentEpochSetup.Participants.Union(previousEpochParticipants)
	// sanity check: size of identities should be equal to previous and current epoch participants combined
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
