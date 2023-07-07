package flow

// DynamicIdentityEntry encapsulates nodeID and dynamic portion of identity.
type DynamicIdentityEntry struct {
	NodeID  Identifier
	Dynamic DynamicIdentity
}

type DynamicIdentityEntryList []*DynamicIdentityEntry

// ProtocolStateEntry is the main structure that will be saved in the database.
// It contains the minimal protocol state that has to be persisted.
type ProtocolStateEntry struct {
	// setup and commit event IDs for current epoch.
	CurrentEpochEventIDs EventIDs
	// setup and commit event IDs for previous epoch.
	PreviousEpochEventIDs EventIDs
	// Part of identity table that can be changed during the epoch.
	Identities DynamicIdentityEntryList
	// InvalidStateTransitionAttempted encodes whether an invalid state transition
	// has been detected in this fork. When this happens, epoch fallback is triggered
	// AFTER the fork is finalized.
	InvalidStateTransitionAttempted bool
	// NextEpochProtocolState describes protocol state of the next epoch
	NextEpochProtocolState *ProtocolStateEntry
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
