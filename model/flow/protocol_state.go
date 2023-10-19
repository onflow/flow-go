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

// ProtocolStateEntry represents a snapshot of the identity table (incl. the set of all notes authorized to
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
	// Setup and commit event IDs for previous epoch.
	PreviousEpochEventIDs EventIDs
	// Setup and commit event IDs for previous epoch. These EventIDs are ZeroID if
	// and only if the current Epoch is the first epoch after a spork or genesis.
	CurrentEpoch EpochStateContainer
	// Protocol state for next epoch. Could be nil if next epoch is not yet set up.
	NextEpoch *EpochStateContainer
	// InvalidStateTransitionAttempted encodes whether an invalid state transition
	// has been detected in this fork. Under normal operations, this value is false.
	// The only possible state transition is false → true. When this happens,
	// epoch fallback is triggered AFTER the fork is finalized.
	InvalidStateTransitionAttempted bool
}

// EpochStateContainer holds the data pertaining to a _single_ epoch but no information about
// any adjacent epochs. To perform a transition from epoch N to N+1, EpochStateContainers for
// both epochs are necessary.
type EpochStateContainer struct {
	// ID of setup event for this epoch, never nil.
	SetupID Identifier
	// ID of commit event for this epoch. Could be ZeroID if epoch was not committed.
	CommitID Identifier
	// ActiveIdentities contains the dynamic identity properties for the nodes that
	// are active in this epoch. Active means that these nodes are authorized to contribute to
	// extending the chain. Nodes are listed in `ActiveIdentities` if and only if
	// they are part of the EpochSetup event for the respective epoch.
	// The dynamic identity properties can change from block to block. Each non-deferred
	// identity-mutating operation is applied independently to the `ActiveIdentities`
	// of the relevant epoch's EpochStateContainer separately.
	// Identities are always sorted in canonical order.
	//
	// Context: In comparison, nodes that are joining in the next epoch or left as of this
	// epoch are only allowed to listen to the network but not actively contribute. Such
	// nodes are _not_ part of `Identities`.
	ActiveIdentities DynamicIdentityEntryList
}

// ID returns an identifier for this EpochStateContainer by hashing internal fields.
// Per convention, the ID of a `nil` EpochStateContainer is `flow.ZeroID`.
func (c *EpochStateContainer) ID() Identifier {
	if c == nil {
		return ZeroID
	}
	return MakeID(c)
}

// EventIDs returns the `flow.EventIDs` with the hashes of the EpochSetup and EpochCommit events.
// Per convention, for a `nil` EpochStateContainer, we return `flow.ZeroID` for both events.
func (c *EpochStateContainer) EventIDs() EventIDs {
	if c == nil {
		return EventIDs{ZeroID, ZeroID}
	}
	return EventIDs{c.SetupID, c.CommitID}
}

// Copy returns a full copy of the entry.
// Embedded Identities are deep-copied, _except_ for their keys, which are copied by reference.
// Per convention, the ID of a `nil` EpochStateContainer is `flow.ZeroID`.
func (c *EpochStateContainer) Copy() *EpochStateContainer {
	if c == nil {
		return nil
	}
	return &EpochStateContainer{
		SetupID:          c.SetupID,
		CommitID:         c.CommitID,
		ActiveIdentities: c.ActiveIdentities.Copy(),
	}
}

// RichProtocolStateEntry is a ProtocolStateEntry which has additional fields that are cached
// from storage layer for convenience.
// Using this structure instead of ProtocolStateEntry allows us to avoid querying
// the database for epoch setups and commits and full identity table.
// It holds several invariants, such as:
//   - CurrentEpochSetup and CurrentEpochCommit are for the same epoch. Never nil.
//   - PreviousEpochSetup and PreviousEpochCommit are for the same epoch. Can be nil.
//   - CurrentEpochIdentityTable is the full (dynamic) identity table for the current epoch.
//     Identities are sorted in canonical order. Without duplicates. Never nil.
//   - NextEpochIdentityTable is the full (dynamic) identity table for the next epoch. Can be nil.
//
// NOTE regarding `CurrentEpochIdentityTable` and `NextEpochIdentityTable`:
// The Identity Table is generally a super-set of the identities listed in the Epoch
// Service Events for the respective epoch. This is because the service events only list
// nodes that are authorized to _actively_ contribute to extending the chain. In contrast,
// the Identity Table additionally contains nodes (with weight zero) from the previous or
// upcoming epoch, which are transitioning into / out of the network and are only allowed
// to listen but not to actively contribute.
type RichProtocolStateEntry struct {
	*ProtocolStateEntry

	PreviousEpochSetup        *EpochSetup
	PreviousEpochCommit       *EpochCommit
	CurrentEpochSetup         *EpochSetup
	CurrentEpochCommit        *EpochCommit
	NextEpochSetup            *EpochSetup
	NextEpochCommit           *EpochCommit
	CurrentEpochIdentityTable IdentityList
	NextEpochIdentityTable    IdentityList
}

// NewRichProtocolStateEntry constructs a rich protocol state entry from a protocol state entry and additional data.
// No errors are expected during normal operation. All errors indicate inconsistent or invalid inputs.
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
		ProtocolStateEntry:        protocolState,
		PreviousEpochSetup:        previousEpochSetup,
		PreviousEpochCommit:       previousEpochCommit,
		CurrentEpochSetup:         currentEpochSetup,
		CurrentEpochCommit:        currentEpochCommit,
		NextEpochSetup:            nextEpochSetup,
		NextEpochCommit:           nextEpochCommit,
		CurrentEpochIdentityTable: IdentityList{},
		NextEpochIdentityTable:    IdentityList{},
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
	if protocolState.CurrentEpoch.SetupID != currentEpochSetup.ID() {
		return nil, fmt.Errorf("supplied current epoch setup (%x) does not match protocol state (%x)",
			currentEpochSetup.ID(),
			protocolState.CurrentEpoch.SetupID)
	}
	if protocolState.CurrentEpoch.CommitID != currentEpochCommit.ID() {
		return nil, fmt.Errorf("supplied current epoch commit (%x) does not match protocol state (%x)",
			currentEpochCommit.ID(),
			protocolState.CurrentEpoch.CommitID)
	}

	var err error
	nextEpoch := protocolState.NextEpoch
	// if next epoch has been already committed, fill in data for it as well.
	if nextEpoch != nil {
		// sanity check consistency of input data
		if nextEpoch.SetupID != nextEpochSetup.ID() {
			return nil, fmt.Errorf("inconsistent EpochSetup for constucting RichProtocolStateEntry, next protocol state states ID %v while input event has ID %v",
				nextEpoch.SetupID, nextEpochSetup.ID())
		}
		if nextEpoch.CommitID != ZeroID {
			if nextEpoch.CommitID != nextEpochCommit.ID() {
				return nil, fmt.Errorf("inconsistent EpochCommit for constucting RichProtocolStateEntry, next protocol state states ID %v while input event has ID %v",
					nextEpoch.CommitID, nextEpochCommit.ID())
			}
		}

		// if next epoch is available, it means that we have observed epoch setup event and we are not anymore in staking phase,
		// so we need to build the identity table using current and next epoch setup events.
		result.CurrentEpochIdentityTable, err = BuildIdentityTable(
			protocolState.CurrentEpoch.ActiveIdentities,
			currentEpochSetup.Participants,
			nextEpochSetup.Participants,
		)
		if err != nil {
			return nil, fmt.Errorf("could not build identity table for setup/commit phase: %w", err)
		}

		result.NextEpochIdentityTable, err = BuildIdentityTable(
			nextEpoch.ActiveIdentities,
			nextEpochSetup.Participants,
			currentEpochSetup.Participants,
		)
		if err != nil {
			return nil, fmt.Errorf("could not build next epoch identity table: %w", err)
		}
	} else {
		// if next epoch is not yet created, it means that we are in staking phase,
		// so we need to build the identity table using previous and current epoch setup events.
		var previousEpochIdentities IdentitySkeletonList
		if previousEpochSetup != nil {
			previousEpochIdentities = previousEpochSetup.Participants
		}
		result.CurrentEpochIdentityTable, err = BuildIdentityTable(
			protocolState.CurrentEpoch.ActiveIdentities,
			currentEpochSetup.Participants,
			previousEpochIdentities,
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
		PreviousEpochEventIDs           EventIDs
		CurrentEpochID                  Identifier
		NextEpochID                     Identifier
		InvalidStateTransitionAttempted bool
	}{
		PreviousEpochEventIDs:           e.PreviousEpochEventIDs,
		CurrentEpochID:                  e.CurrentEpoch.ID(),
		NextEpochID:                     e.NextEpoch.ID(),
		InvalidStateTransitionAttempted: e.InvalidStateTransitionAttempted,
	}
	return MakeID(body)
}

// Copy returns a full copy of the entry.
// Embedded Identities are deep-copied, _except_ for their keys, which are copied by reference.
func (e *ProtocolStateEntry) Copy() *ProtocolStateEntry {
	if e == nil {
		return nil
	}
	return &ProtocolStateEntry{
		PreviousEpochEventIDs:           e.PreviousEpochEventIDs,
		CurrentEpoch:                    *e.CurrentEpoch.Copy(),
		NextEpoch:                       e.NextEpoch.Copy(),
		InvalidStateTransitionAttempted: e.InvalidStateTransitionAttempted,
	}
}

// Copy returns a full copy of rich protocol state entry.
//   - Embedded service events are copied by reference (not deep-copied).
//   - CurrentEpochIdentityTable and NextEpochIdentityTable are deep-copied, _except_ for their keys, which are copied by reference.
func (e *RichProtocolStateEntry) Copy() *RichProtocolStateEntry {
	if e == nil {
		return nil
	}
	return &RichProtocolStateEntry{
		ProtocolStateEntry:        e.ProtocolStateEntry.Copy(),
		PreviousEpochSetup:        e.PreviousEpochSetup,
		PreviousEpochCommit:       e.PreviousEpochCommit,
		CurrentEpochSetup:         e.CurrentEpochSetup,
		CurrentEpochCommit:        e.CurrentEpochCommit,
		NextEpochSetup:            e.NextEpochSetup,
		NextEpochCommit:           e.NextEpochCommit,
		CurrentEpochIdentityTable: e.CurrentEpochIdentityTable.Copy(),
		NextEpochIdentityTable:    e.NextEpochIdentityTable.Copy(),
	}
}

// EpochStatus returns epoch status for the current protocol state.
func (e *ProtocolStateEntry) EpochStatus() *EpochStatus {
	return &EpochStatus{
		PreviousEpoch:                   e.PreviousEpochEventIDs,
		CurrentEpoch:                    e.CurrentEpoch.EventIDs(),
		NextEpoch:                       e.NextEpoch.EventIDs(),
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
	for i := 0; i+1 < len(ll); i++ {
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

// Copy returns a copy of the DynamicIdentityEntryList. The resulting slice uses
// a different backing array, meaning appends and insert operations on either slice
// are guaranteed to only affect that slice.
//
// Copy should be used when modifying an existing identity list by either
// appending new elements, re-ordering, or inserting new elements in an
// existing index.
//
// CAUTION:
// All Identity fields are deep-copied, _except_ for their keys, which
// are copied by reference.
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
// CAUTION:
// All Identity fields are deep-copied, _except_ for their keys, which are copied by reference.
func (ll DynamicIdentityEntryList) Sort(less IdentifierOrder) DynamicIdentityEntryList {
	dup := ll.Copy()
	sort.Slice(dup, func(i int, j int) bool {
		return less(dup[i].NodeID, dup[j].NodeID)
	})
	return dup
}

// BuildIdentityTable constructs the full identity table for the target epoch by combining data from:
//  1. The Dynamic Identities for the nodes that are _active_ in the target epoch (i.e. the dynamic identity
//     fields for the IdentitySkeletons contained in the EpochSetup event for the respective epoch).
//  2. The IdentitySkeletons for the nodes that are _active_ in the target epoch
//     (recorded in EpochSetup event and immutable throughout the epoch).
//  3. [optional] An adjacent epoch's IdentitySkeletons (can be empty or nil), as recorded in the
//     adjacent epoch's setup event. For a target epoch N, the epochs N-1 and N+1 are defined to be
//     adjacent. Adjacent epochs do not _necessarily_ exist (e.g. consider a spork comprising only
//     a single epoch), in which case this input is nil or empty.
//
// It also performs sanity checks to make sure that the data is consistent.
// No errors are expected during normal operation. All errors indicate inconsistent or invalid inputs.
func BuildIdentityTable(
	targetEpochDynamicIdentities DynamicIdentityEntryList,
	targetEpochIdentitySkeletons IdentitySkeletonList,
	adjacentEpochIdentitySkeletons IdentitySkeletonList,
) (IdentityList, error) {
	// Combine the participants of the current and adjacent epoch. The method `GenericIdentityList.Union`
	// already implements the following required conventions:
	//  1. Preference for IdentitySkeleton of the target epoch:
	//     In case an IdentitySkeleton with the same NodeID exists in the target epoch as well as
	//     in the adjacent epoch, we use the IdentitySkeleton for the target epoch (for example,
	//     to account for changes of keys, address, initial weight, etc).
	//  2. Canonical ordering
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
			IdentitySkeleton: *allEpochParticipants[i],
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
