package flow

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
// - CurrentEpochSetup and CurrentEpochCommit are for the same epoch. Never nil.
// - PreviousEpochSetup and PreviousEpochCommit are for the same epoch. Never nil.
// - Identities is a full identity table for the current epoch. Identities are sorted in canonical order. Never nil.
// - NextEpochProtocolState is a protocol state for the next epoch. Can be nil.
type RichProtocolStateEntry struct {
	ProtocolStateEntry

	CurrentEpochSetup   *EpochSetup
	CurrentEpochCommit  *EpochCommit
	PreviousEpochSetup  *EpochSetup
	PreviousEpochCommit *EpochCommit
	Identities          IdentityList

	NextEpochProtocolState *RichProtocolStateEntry
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
	identities := make(DynamicIdentityEntryList, 0, len(e.Identities))
	for _, identity := range e.Identities {
		identities = append(identities, &DynamicIdentityEntry{
			NodeID:  identity.NodeID,
			Dynamic: identity.Dynamic,
		})
	}
	return &ProtocolStateEntry{
		CurrentEpochEventIDs:            e.CurrentEpochEventIDs,
		PreviousEpochEventIDs:           e.PreviousEpochEventIDs,
		Identities:                      identities,
		InvalidStateTransitionAttempted: e.InvalidStateTransitionAttempted,
		NextEpochProtocolState:          e.NextEpochProtocolState.Copy(),
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
