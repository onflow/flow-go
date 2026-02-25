package flow

import (
	"fmt"

	"golang.org/x/exp/slices"
)

// DynamicIdentityEntry encapsulates nodeID and dynamic portion of identity.
type DynamicIdentityEntry struct {
	NodeID  Identifier
	Ejected bool
}

// EqualTo returns true if the two DynamicIdentityEntry are equivalent.
func (d *DynamicIdentityEntry) EqualTo(other *DynamicIdentityEntry) bool {
	// Shortcut if `t` and `other` point to the same object; covers case where both are nil.
	if d == other {
		return true
	}
	if d == nil || other == nil { // only one is nil, the other not (otherwise we would have returned above)
		return false
	}

	return d.NodeID == other.NodeID &&
		d.Ejected == other.Ejected
}

type DynamicIdentityEntryList []*DynamicIdentityEntry

// MinEpochStateEntry is the most compact snapshot of the epoch state and identity table (set of all notes authorized to
// be part of the network) at some specific block. This struct is optimized for persisting the identity table
// in the database, in that it only includes data that is variable during the course of an epoch to avoid
// storage of redundant data. The Epoch Setup and Commit events, which carry the portion of the identity
// table that is constant throughout an epoch, are only referenced by their hash commitment.
// Note that a MinEpochStateEntry does not hold the entire data for the identity table directly. It
// allows reconstructing the identity table with the referenced epoch setup events and dynamic identities.
//
//structwrite:immutable - mutations allowed only within the constructor
type MinEpochStateEntry struct {
	PreviousEpoch *EpochStateContainer // minimal dynamic properties for previous epoch [optional, nil for first epoch after spork, genesis]
	CurrentEpoch  EpochStateContainer  // minimal dynamic properties for current epoch
	NextEpoch     *EpochStateContainer // minimal dynamic properties for next epoch [optional, nil iff we are in staking phase]

	// EpochFallbackTriggered encodes whether an invalid epoch transition
	// has been detected in this fork. Under normal operations, this value is false.
	// Node-internally, the EpochFallback notification is emitted when a block is
	// finalized that changes this flag from false to true.
	// A state transition from true -> false is possible only when protocol undergoes epoch recovery.
	EpochFallbackTriggered bool
}

// UntrustedMinEpochStateEntry is an untrusted input-only representation of a MinEpochStateEntry,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedMinEpochStateEntry should be validated and converted into
// a trusted MinEpochStateEntry using NewMinEpochStateEntry constructor.
type UntrustedMinEpochStateEntry MinEpochStateEntry

// NewMinEpochStateEntry creates a new instance of MinEpochStateEntry.
// Construction MinEpochStateEntry allowed only within the constructor.
//
// All errors indicate a valid MinEpochStateEntry cannot be constructed from the input.
func NewMinEpochStateEntry(untrusted UntrustedMinEpochStateEntry) (*MinEpochStateEntry, error) {
	if untrusted.CurrentEpoch.EqualTo(new(EpochStateContainer)) {
		return nil, fmt.Errorf("current epoch must not be empty")
	}
	return &MinEpochStateEntry{
		PreviousEpoch:          untrusted.PreviousEpoch,
		CurrentEpoch:           untrusted.CurrentEpoch,
		NextEpoch:              untrusted.NextEpoch,
		EpochFallbackTriggered: untrusted.EpochFallbackTriggered,
	}, nil
}

// EpochStateContainer holds the data pertaining to a _single_ epoch but no information about
// any adjacent epochs. To perform a transition from epoch N to N+1, EpochStateContainers for
// both epochs are necessary.
//
//structwrite:immutable - mutations allowed only within the constructor
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

	// EpochExtensions contains potential EFM-extensions of this epoch. In the happy path
	// it is nil or empty. An Epoch in which Epoch-Fallback-Mode [EFM] is triggered, will
	// have at least one extension. By convention, the initial extension must satisfy
	//   EpochSetup.FinalView + 1 = EpochExtensions[0].FirstView
	// and each consecutive pair of slice elements must obey
	//   EpochExtensions[i].FinalView+1 = EpochExtensions[i+1].FirstView
	EpochExtensions []EpochExtension
}

// UntrustedEpochStateContainer is an untrusted input-only representation of a EpochStateContainer,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedEpochStateContainer should be validated and converted into
// a trusted EpochStateContainer using NewEpochStateContainer constructor.
type UntrustedEpochStateContainer EpochStateContainer

// NewEpochStateContainer creates a new instance of EpochStateContainer.
// Construction EpochStateContainer allowed only within the constructor.
//
// All errors indicate a valid EpochStateContainer cannot be constructed from the input.
func NewEpochStateContainer(untrusted UntrustedEpochStateContainer) (*EpochStateContainer, error) {
	if untrusted.SetupID == ZeroID {
		return nil, fmt.Errorf("SetupID must not be zero")
	}
	if untrusted.ActiveIdentities == nil {
		return nil, fmt.Errorf("ActiveIdentities must not be nil")
	}
	if !untrusted.ActiveIdentities.Sorted(IdentifierCanonical) {
		return nil, fmt.Errorf("ActiveIdentities are not sorted")
	}

	return &EpochStateContainer{
		SetupID:          untrusted.SetupID,
		CommitID:         untrusted.CommitID,
		ActiveIdentities: untrusted.ActiveIdentities,
		EpochExtensions:  untrusted.EpochExtensions,
	}, nil
}

// EpochExtension represents a range of views, which contiguously extends this epoch.
type EpochExtension struct {
	FirstView uint64
	FinalView uint64
}

// EqualTo returns true if the two EpochExtension are equivalent.
func (e *EpochExtension) EqualTo(other *EpochExtension) bool {
	// Shortcut if `t` and `other` point to the same object; covers case where both are nil.
	if e == other {
		return true
	}
	if e == nil || other == nil { // only one is nil, the other not (otherwise we would have returned above)
		return false
	}

	return e.FirstView == other.FirstView &&
		e.FinalView == other.FinalView
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
	var ext []EpochExtension
	if c.EpochExtensions != nil {
		ext = make([]EpochExtension, len(c.EpochExtensions))
		copy(ext, c.EpochExtensions)
	}

	// Constructor is skipped since we're copying an already-valid object.
	//nolint:structwrite
	return &EpochStateContainer{
		SetupID:          c.SetupID,
		CommitID:         c.CommitID,
		ActiveIdentities: c.ActiveIdentities.Copy(),
		EpochExtensions:  ext,
	}
}

// EqualTo returns true if the two EpochStateContainer are equivalent.
func (c *EpochStateContainer) EqualTo(other *EpochStateContainer) bool {
	// Shortcut if `t` and `other` point to the same object; covers case where both are nil.
	if c == other {
		return true
	}
	if c == nil || other == nil { // only one is nil, the other not (otherwise we would have returned above)
		return false
	}
	// both are not nil, so we can compare the fields
	if c.SetupID != other.SetupID {
		return false
	}
	if c.CommitID != other.CommitID {
		return false
	}
	if !slices.EqualFunc(c.ActiveIdentities, other.ActiveIdentities, func(e1 *DynamicIdentityEntry, e2 *DynamicIdentityEntry) bool {
		return e1.EqualTo(e2)
	}) {
		return false
	}

	if !slices.EqualFunc(c.EpochExtensions, other.EpochExtensions, func(e1 EpochExtension, e2 EpochExtension) bool {
		return e1.EqualTo(&e2)
	}) {
		return false
	}

	return true
}

// EpochStateEntry is a MinEpochStateEntry that has additional fields that are cached from the
// storage layer for convenience. It holds all the information needed to construct a snapshot of
// the identity table (set of all notes authorized to be part of the network) at some specific
// block without any database queries. Specifically, `MinEpochStateEntry` is a snapshot of the
// variable portion of the identity table. The portion of the identity table that is constant
// throughout an epoch is contained in the Epoch Setup and Epoch Commit events.
// Convention:
//   - CurrentEpochSetup and CurrentEpochCommit are for the same epoch. Never nil.
//   - PreviousEpochSetup and PreviousEpochCommit are for the same epoch. Can be nil.
//   - NextEpochSetup and NextEpochCommit are for the same epoch. Can be nil.
//
//structwrite:immutable - mutations allowed only within the constructor.
type EpochStateEntry struct {
	*MinEpochStateEntry

	// by convention, all epoch service events are immutable
	PreviousEpochSetup  *EpochSetup
	PreviousEpochCommit *EpochCommit
	CurrentEpochSetup   *EpochSetup
	CurrentEpochCommit  *EpochCommit
	NextEpochSetup      *EpochSetup
	NextEpochCommit     *EpochCommit
}

// UntrustedEpochStateEntry is an untrusted input-only representation of an EpochStateEntry,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedEpochStateEntry should be validated and converted into
// a trusted EpochStateEntry using NewEpochStateEntry constructor.
type UntrustedEpochStateEntry EpochStateEntry

// NewEpochStateEntry constructs an EpochStateEntry from a MinEpochStateEntry and additional data.
//
// All errors indicate a valid EpochStateEntry cannot be constructed from the input.
func NewEpochStateEntry(untrusted UntrustedEpochStateEntry) (*EpochStateEntry, error) {
	// If previous epoch is specified: ensure respective epoch service events are not nil and consistent with commitments in `MinEpochStateEntry.PreviousEpoch`
	if untrusted.PreviousEpoch != nil {
		if untrusted.PreviousEpoch.SetupID != untrusted.PreviousEpochSetup.ID() { // calling ID() will panic is EpochSetup event is nil
			return nil, fmt.Errorf("supplied previous epoch's setup event (%x) does not match commitment (%x) in MinEpochStateEntry", untrusted.PreviousEpochSetup.ID(), untrusted.PreviousEpoch.SetupID)
		}
		if untrusted.PreviousEpoch.CommitID != untrusted.PreviousEpochCommit.ID() { // calling ID() will panic is EpochCommit event is nil
			return nil, fmt.Errorf("supplied previous epoch's commit event (%x) does not match commitment (%x) in MinEpochStateEntry", untrusted.PreviousEpochCommit.ID(), untrusted.PreviousEpoch.CommitID)
		}
	} else {
		if untrusted.PreviousEpochSetup != nil {
			return nil, fmt.Errorf("no previous epoch but gotten non-nil EpochSetup event")
		}
		if untrusted.PreviousEpochCommit != nil {
			return nil, fmt.Errorf("no previous epoch but gotten non-nil EpochCommit event")
		}
	}

	// For current epoch: ensure respective epoch service events are not nil and consistent with commitments in `MinEpochStateEntry.CurrentEpoch`
	if untrusted.CurrentEpoch.SetupID != untrusted.CurrentEpochSetup.ID() { // calling ID() will panic is EpochSetup event is nil
		return nil, fmt.Errorf("supplied current epoch's setup event (%x) does not match commitment (%x) in MinEpochStateEntry", untrusted.CurrentEpochSetup.ID(), untrusted.CurrentEpoch.SetupID)
	}
	if untrusted.CurrentEpoch.CommitID != untrusted.CurrentEpochCommit.ID() { // calling ID() will panic is EpochCommit event is nil
		return nil, fmt.Errorf("supplied current epoch's commit event (%x) does not match commitment (%x) in MinEpochStateEntry", untrusted.CurrentEpochCommit.ID(), untrusted.CurrentEpoch.CommitID)
	}

	// If we are in staking phase (i.e. epochState.NextEpoch == nil):
	//  (1) Full identity table contains active identities from current epoch.
	//      If previous epoch exists, we add nodes from previous epoch that are leaving in the current epoch with `EpochParticipationStatusLeaving` status.
	// Otherwise, we are in epoch setup or epoch commit phase (i.e. epochState.NextEpoch ≠ nil):
	//  (2a) Full identity table contains active identities from current epoch + nodes joining in next epoch with `EpochParticipationStatusJoining` status.
	//  (2b) Furthermore, we also build the full identity table for the next epoch's staking phase:
	//       active identities from next epoch + nodes from current epoch that are leaving at the end of the current epoch with `flow.EpochParticipationStatusLeaving` status.
	nextEpoch := untrusted.NextEpoch
	if nextEpoch == nil { // in staking phase: build full identity table for current epoch according to (1)
		if untrusted.NextEpochSetup != nil {
			return nil, fmt.Errorf("no next epoch but gotten non-nil EpochSetup event")
		}
		if untrusted.NextEpochCommit != nil {
			return nil, fmt.Errorf("no next epoch but gotten non-nil EpochCommit event")
		}
	} else { // epochState.NextEpoch ≠ nil, i.e. we are in epoch setup or epoch commit phase
		// ensure respective epoch service events are not nil and consistent with commitments in `MinEpochStateEntry.NextEpoch`
		if nextEpoch.SetupID != untrusted.NextEpochSetup.ID() {
			return nil, fmt.Errorf("supplied next epoch's setup event (%x) does not match commitment (%x) in MinEpochStateEntry", nextEpoch.SetupID, untrusted.NextEpochSetup.ID())
		}
		if nextEpoch.CommitID != ZeroID {
			if nextEpoch.CommitID != untrusted.NextEpochCommit.ID() {
				return nil, fmt.Errorf("supplied next epoch's commit event (%x) does not match commitment (%x) in MinEpochStateEntry", nextEpoch.CommitID, untrusted.NextEpochCommit.ID())
			}
		} else {
			if untrusted.NextEpochCommit != nil {
				return nil, fmt.Errorf("next epoch not yet committed but got EpochCommit event")
			}
		}
	}
	return &EpochStateEntry{
		MinEpochStateEntry:  untrusted.MinEpochStateEntry,
		PreviousEpochSetup:  untrusted.PreviousEpochSetup,
		PreviousEpochCommit: untrusted.PreviousEpochCommit,
		CurrentEpochSetup:   untrusted.CurrentEpochSetup,
		CurrentEpochCommit:  untrusted.CurrentEpochCommit,
		NextEpochSetup:      untrusted.NextEpochSetup,
		NextEpochCommit:     untrusted.NextEpochCommit,
	}, nil
}

// RichEpochStateEntry is a EpochStateEntry that additionally holds the canonical representation of the
// identity table (set of all notes authorized to be part of the network) at some specific block.
// This data structure is optimized for frequent reads of the same identity table, which is
// the prevalent case during normal operations (node ejections and epoch fallback are rare).
// Conventions:
//   - Invariants inherited from EpochStateEntry.
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
//
//structwrite:immutable - mutations allowed only within the constructor
type RichEpochStateEntry struct {
	*EpochStateEntry

	CurrentEpochIdentityTable IdentityList
	NextEpochIdentityTable    IdentityList
}

// NewRichEpochStateEntry constructs a RichEpochStateEntry from an EpochStateEntry.
// Construction RichEpochStateEntry allowed only within the constructor.
//
// All errors indicate a valid RichEpochStateEntry cannot be constructed from the input.
func NewRichEpochStateEntry(epochState *EpochStateEntry) (*RichEpochStateEntry, error) {
	if epochState == nil {
		return nil, fmt.Errorf("epoch state must not be nil")
	}
	var currentEpochIdentityTable IdentityList
	nextEpochIdentityTable := IdentityList{}
	// If we are in staking phase (i.e. epochState.NextEpoch == nil):
	//  (1) Full identity table contains active identities from current epoch.
	//      If previous epoch exists, we add nodes from previous epoch that are leaving in the current epoch with status `EpochParticipationStatusLeaving`.
	// Otherwise, we are in epoch setup or epoch commit phase (i.e. epochState.NextEpoch ≠ nil):
	//  (2a) Full identity table contains active identities from current epoch + nodes joining in next epoch with status `EpochParticipationStatusJoining`.
	//  (2b) Furthermore, we also build the full identity table for the next epoch's staking phase:
	//       active identities from next epoch + nodes from current epoch that are leaving at the end of the current epoch with `flow.EpochParticipationStatusLeaving` status.
	var err error
	nextEpoch := epochState.NextEpoch
	if nextEpoch == nil { // in staking phase: build full identity table for current epoch according to (1)
		var previousEpochIdentitySkeletons IdentitySkeletonList
		var previousEpochDynamicIdentities DynamicIdentityEntryList
		if previousEpochSetup := epochState.PreviousEpochSetup; previousEpochSetup != nil {
			previousEpochIdentitySkeletons = previousEpochSetup.Participants
			previousEpochDynamicIdentities = epochState.PreviousEpoch.ActiveIdentities
		}
		currentEpochIdentityTable, err = BuildIdentityTable(
			epochState.CurrentEpochSetup.Participants,
			epochState.CurrentEpoch.ActiveIdentities,
			previousEpochIdentitySkeletons,
			previousEpochDynamicIdentities,
			EpochParticipationStatusLeaving,
		)
		if err != nil {
			return nil, fmt.Errorf("could not build identity table for staking phase: %w", err)
		}
	} else { // epochState.NextEpoch ≠ nil, i.e. we are in epoch setup or epoch commit phase
		currentEpochIdentityTable, err = BuildIdentityTable(
			epochState.CurrentEpochSetup.Participants,
			epochState.CurrentEpoch.ActiveIdentities,
			epochState.NextEpochSetup.Participants,
			nextEpoch.ActiveIdentities,
			EpochParticipationStatusJoining,
		)
		if err != nil {
			return nil, fmt.Errorf("could not build identity table for setup/commit phase: %w", err)
		}

		nextEpochIdentityTable, err = BuildIdentityTable(
			epochState.NextEpochSetup.Participants,
			nextEpoch.ActiveIdentities,
			epochState.CurrentEpochSetup.Participants,
			epochState.CurrentEpoch.ActiveIdentities,
			EpochParticipationStatusLeaving,
		)
		if err != nil {
			return nil, fmt.Errorf("could not build next epoch identity table: %w", err)
		}
	}

	return &RichEpochStateEntry{
		EpochStateEntry:           epochState,
		CurrentEpochIdentityTable: currentEpochIdentityTable,
		NextEpochIdentityTable:    nextEpochIdentityTable,
	}, nil
}

// ID returns hash of entry by hashing all fields.
func (e *MinEpochStateEntry) ID() Identifier {
	if e == nil {
		return ZeroID
	}
	return MakeID(e)
}

// Copy returns a full copy of the entry.
// Embedded Identities are deep-copied, _except_ for their keys, which are copied by reference.
func (e *MinEpochStateEntry) Copy() *MinEpochStateEntry {
	if e == nil {
		return nil
	}
	// Constructor is skipped since we're copying an already-valid object.
	//nolint:structwrite
	return &MinEpochStateEntry{
		PreviousEpoch:          e.PreviousEpoch.Copy(),
		CurrentEpoch:           *e.CurrentEpoch.Copy(),
		NextEpoch:              e.NextEpoch.Copy(),
		EpochFallbackTriggered: e.EpochFallbackTriggered,
	}
}

// Copy returns a full copy of the EpochStateEntry.
//   - Embedded service events are copied by reference (not deep-copied).
func (e *EpochStateEntry) Copy() *EpochStateEntry {
	if e == nil {
		return nil
	}

	// Constructor is skipped since we're copying an already-valid object.
	//nolint:structwrite
	return &EpochStateEntry{
		MinEpochStateEntry:  e.MinEpochStateEntry.Copy(),
		PreviousEpochSetup:  e.PreviousEpochSetup,
		PreviousEpochCommit: e.PreviousEpochCommit,
		CurrentEpochSetup:   e.CurrentEpochSetup,
		CurrentEpochCommit:  e.CurrentEpochCommit,
		NextEpochSetup:      e.NextEpochSetup,
		NextEpochCommit:     e.NextEpochCommit,
	}
}

// Copy returns a full copy of the RichEpochStateEntry.
//   - Embedded service events are copied by reference (not deep-copied).
//   - CurrentEpochIdentityTable and NextEpochIdentityTable are deep-copied, _except_ for their keys, which are copied by reference.
func (e *RichEpochStateEntry) Copy() *RichEpochStateEntry {
	if e == nil {
		return nil
	}
	// Constructor is skipped since we're copying an already-valid object.
	//nolint:structwrite
	return &RichEpochStateEntry{
		EpochStateEntry:           e.EpochStateEntry.Copy(),
		CurrentEpochIdentityTable: e.CurrentEpochIdentityTable.Copy(),
		NextEpochIdentityTable:    e.NextEpochIdentityTable.Copy(),
	}
}

// CurrentEpochFinalView returns the final view of the current epoch, taking into account possible epoch extensions.
// If there are no epoch extensions, the final view is the final view of the current epoch setup,
// otherwise it is the final view of the last epoch extension.
func (e *EpochStateEntry) CurrentEpochFinalView() uint64 {
	l := len(e.CurrentEpoch.EpochExtensions)
	if l > 0 {
		return e.CurrentEpoch.EpochExtensions[l-1].FinalView
	}
	return e.CurrentEpochSetup.FinalView
}

// EpochPhase returns the current epoch phase.
// The receiver MinEpochStateEntry must be properly constructed.
// See flow.EpochPhase for detailed documentation.
func (e *MinEpochStateEntry) EpochPhase() EpochPhase {
	// CAUTION: the logic below that deduces the EpochPhase must be consistent with `epochs.FallbackStateMachine`,
	// which sets the fields we are using here. Specifically, we require that the FallbackStateMachine clears out
	// any tentative values for a subsequent epoch _unless_ that epoch is already committed.
	if e.EpochFallbackTriggered {
		// If the next epoch has been committed, we are in EpochPhaseCommitted regardless of EFM status.
		// We will enter EpochPhaseFallback after completing the transition into the committed next epoch.
		if e.NextEpoch != nil && e.NextEpoch.CommitID != ZeroID {
			return EpochPhaseCommitted
		}
		// If the next epoch has not been committed and EFM is triggered, we immediately enter EpochPhaseFallback.
		return EpochPhaseFallback
	}

	// The epoch phase is determined by how much information we have about the next epoch
	if e.NextEpoch == nil {
		return EpochPhaseStaking // if no information about the next epoch is known, we are in the Staking Phase
	}
	// Per convention, NextEpoch ≠ nil if and only if NextEpoch.SetupID is specified.
	if e.NextEpoch.CommitID == ZeroID {
		return EpochPhaseSetup // if only the Setup event is known for the next epoch but not the Commit event, we are in the Setup Phase
	}
	return EpochPhaseCommitted // if the Setup and Commit events are known for the next epoch, we are in the Committed Phase
}

// EpochCounter returns the current epoch counter.
// The receiver RichEpochStateEntry must be properly constructed.
func (e *EpochStateEntry) EpochCounter() uint64 {
	return e.CurrentEpochSetup.Counter
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
	return slices.IsSortedFunc(ll, func(lhs, rhs *DynamicIdentityEntry) int {
		return less(lhs.NodeID, rhs.NodeID)
	})
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
	lenList := len(ll)
	dup := make(DynamicIdentityEntryList, 0, lenList)
	for i := range lenList {
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
	slices.SortFunc(dup, func(lhs, rhs *DynamicIdentityEntry) int {
		return less(lhs.NodeID, rhs.NodeID)
	})
	return dup
}

// BuildIdentityTable constructs the full identity table for the target epoch by combining data from:
//  1. The IdentitySkeletons for the nodes that are _active_ in the target epoch
//     (recorded in EpochSetup event and immutable throughout the epoch).
//  2. The Dynamic Identities for the nodes that are _active_ in the target epoch (i.e. the dynamic identity
//     fields for the IdentitySkeletons contained in the EpochSetup event for the respective epoch).
//
// Optionally, identity information for an adjacent epoch is given if and only if an adjacent epoch exists. For
// a target epoch N, the epochs N-1 and N+1 are defined to be adjacent. Adjacent epochs do not necessarily exist
// (e.g. consider a spork comprising only a single epoch), in which case the respective inputs are nil or empty.
//  3. [optional] An adjacent epoch's IdentitySkeletons as recorded in the adjacent epoch's setup event.
//  4. [optional] An adjacent epoch's Dynamic Identities.
//  5. An adjacent epoch's identities participation status, this could be joining or leaving depending on epoch phase.
//
// The function enforces that the input slices pertaining to the same epoch contain the same identities
// (compared by nodeID) in the same order. Otherwise, an exception is returned.
// No errors are expected during normal operation. All errors indicate inconsistent or invalid inputs.
func BuildIdentityTable(
	targetEpochIdentitySkeletons IdentitySkeletonList,
	targetEpochDynamicIdentities DynamicIdentityEntryList,
	adjacentEpochIdentitySkeletons IdentitySkeletonList,
	adjacentEpochDynamicIdentities DynamicIdentityEntryList,
	adjacentIdentitiesStatus EpochParticipationStatus,
) (IdentityList, error) {
	if adjacentIdentitiesStatus != EpochParticipationStatusLeaving &&
		adjacentIdentitiesStatus != EpochParticipationStatusJoining {
		return nil, fmt.Errorf("invalid adjacent identity status, expect %s or %s, got %s",
			EpochParticipationStatusLeaving.String(),
			EpochParticipationStatusJoining.String(),
			adjacentIdentitiesStatus)
	}
	targetEpochParticipants, err := ComposeFullIdentities(targetEpochIdentitySkeletons, targetEpochDynamicIdentities, EpochParticipationStatusActive)
	if err != nil {
		return nil, fmt.Errorf("could not reconstruct participants for target epoch: %w", err)
	}
	adjacentEpochParticipants, err := ComposeFullIdentities(adjacentEpochIdentitySkeletons, adjacentEpochDynamicIdentities, adjacentIdentitiesStatus)
	if err != nil {
		return nil, fmt.Errorf("could not reconstruct participants for adjacent epoch: %w", err)
	}

	// Combine the participants of the current and adjacent epoch. The method `GenericIdentityList.Union`
	// already implements the following required conventions:
	//  1. Preference for IdentitySkeleton of the target epoch:
	//     In case an IdentitySkeleton with the same NodeID exists in the target epoch as well as
	//     in the adjacent epoch, we use the IdentitySkeleton for the target epoch (for example,
	//     to account for changes of keys, address, initial weight, etc).
	//  2. Canonical ordering
	return targetEpochParticipants.Union(adjacentEpochParticipants), nil
}

// DynamicIdentityEntryListFromIdentities converts IdentityList to DynamicIdentityEntryList.
func DynamicIdentityEntryListFromIdentities(identities IdentityList) DynamicIdentityEntryList {
	dynamicIdentities := make(DynamicIdentityEntryList, 0, len(identities))
	for _, identity := range identities {
		dynamicIdentities = append(dynamicIdentities, &DynamicIdentityEntry{
			NodeID:  identity.NodeID,
			Ejected: identity.IsEjected(),
		})
	}
	return dynamicIdentities
}

// ComposeFullIdentities combines identity skeletons and dynamic identities to produce a flow.IdentityList.
// It enforces that the input slices `skeletons` and `dynamics` list the same identities (compared by nodeID)
// in the same order. Otherwise, an exception is returned. For each identity i, we set
// `i.EpochParticipationStatus` to the `defaultEpochParticipationStatus` _unless_ i is ejected.
// No errors are expected during normal operations.
func ComposeFullIdentities(
	skeletons IdentitySkeletonList,
	dynamics DynamicIdentityEntryList,
	defaultEpochParticipationStatus EpochParticipationStatus,
) (IdentityList, error) {
	// sanity check: list of skeletons and dynamic should be the same
	if len(skeletons) != len(dynamics) {
		return nil, fmt.Errorf("invalid number of identities to reconstruct: expected %d, got %d", len(skeletons), len(dynamics))
	}

	// reconstruct identities from skeleton and dynamic parts
	var result IdentityList
	for i := range dynamics {
		// sanity check: identities should be sorted in the same order
		if dynamics[i].NodeID != skeletons[i].NodeID {
			return nil, fmt.Errorf("identites in protocol state are not consistently ordered: expected %s, got %s", skeletons[i].NodeID, dynamics[i].NodeID)
		}
		status := defaultEpochParticipationStatus
		if dynamics[i].Ejected {
			status = EpochParticipationStatusEjected
		}
		result = append(result, &Identity{
			IdentitySkeleton: *skeletons[i],
			DynamicIdentity: DynamicIdentity{
				EpochParticipationStatus: status,
			},
		})
	}
	return result, nil
}

// PSKeyValueStoreData is a binary blob with a version attached, specifying the format
// of the marshaled data. In a nutshell, it serves as a binary snapshot of a ProtocolKVStore.
// This structure is useful for version-agnostic storage, where snapshots with different versions
// can co-exist. The PSKeyValueStoreData is a generic format that can be later decoded to
// potentially different strongly typed structures based on version. When reading from the store,
// callers must know how to deal with the binary representation.
type PSKeyValueStoreData struct {
	Version uint64
	Data    []byte
}

func (d PSKeyValueStoreData) Equal(d2 *PSKeyValueStoreData) bool {
	return d.Version == d2.Version &&
		slices.Equal(d.Data, d2.Data)
}

// VersionedInstanceParams is a binary instance params blob with a version attached, specifying the format
// of the marshaled data. In a nutshell, it serves as a binary of an InstanceParams.
// The VersionedInstanceParams is a generic format that can be later decoded to
// potentially different strongly typed structures based on version. When reading from the store,
// callers must know how to deal with the binary representation.
type VersionedInstanceParams struct {
	Version uint64
	Data    []byte
}
