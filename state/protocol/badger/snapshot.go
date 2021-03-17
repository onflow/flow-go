// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/mapfunc"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/seed"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
)

// Snapshot implements the protocol.Snapshot interface.
// It represents a read-only immutable snapshot of the protocol state at the
// block it is constructed with. It allows efficient access to data associated directly
// with blocks at a given state (finalized, sealed), such as the related header, commit,
// seed or pending children. A block snapshot can lazily convert to an epoch snapshot in
// order to make data associated directly with epochs accessible through its API.
type Snapshot struct {
	state   *State
	blockID flow.Identifier // reference block for this snapshot
}

func NewSnapshot(state *State, blockID flow.Identifier) *Snapshot {
	return &Snapshot{
		state:   state,
		blockID: blockID,
	}
}

func (s *Snapshot) Head() (*flow.Header, error) {
	head, err := s.state.headers.ByBlockID(s.blockID)
	return head, err
}

func (s *Snapshot) Phase() (flow.EpochPhase, error) {
	status, err := s.state.epoch.statuses.ByBlockID(s.blockID)
	if err != nil {
		return flow.EpochPhaseUndefined, fmt.Errorf("could not retrieve epoch status: %w", err)
	}
	phase, err := status.Phase()
	return phase, err
}

func (s *Snapshot) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {

	// TODO: CAUTION SHORTCUT
	// we retrieve identities based on the initial identity table from the EpochSetup
	// event here -- this will need revision to support mid-epoch identity changes
	// once slashing is implemented

	status, err := s.state.epoch.statuses.ByBlockID(s.blockID)
	if err != nil {
		return nil, err
	}

	setup, err := s.state.epoch.setups.ByID(status.CurrentEpoch.SetupID)
	if err != nil {
		return nil, err
	}

	// get identities from the current epoch first
	identities := setup.Participants.Copy()
	lookup := identities.Lookup()

	// get identities that are in either last/next epoch but NOT in the current epoch
	var otherEpochIdentities flow.IdentityList
	phase, err := status.Phase()
	if err != nil {
		return nil, fmt.Errorf("could not get phase: %w", err)
	}
	switch phase {
	// during staking phase (the beginning of the epoch) we include identities
	// from the previous epoch that are now un-staking
	case flow.EpochPhaseStaking:

		first, err := s.state.AtBlockID(status.FirstBlockID).Head()
		if err != nil {
			return nil, fmt.Errorf("could not get first block of epoch: %w", err)
		}
		// check whether this is the first epoch after the root block - in this
		// case there are no previous epoch identities to check anyway
		root, err := s.state.Params().Root()
		if err != nil {
			return nil, fmt.Errorf("could not get root block: %w", err)
		}
		if first.Height == root.Height {
			break
		}

		lastStatus, err := s.state.epoch.statuses.ByBlockID(first.ParentID)
		if err != nil {
			return nil, fmt.Errorf("could not get last epoch status: %w", err)
		}
		lastSetup, err := s.state.epoch.setups.ByID(lastStatus.CurrentEpoch.SetupID)
		if err != nil {
			return nil, fmt.Errorf("could not get last epoch setup event: %w", err)
		}

		for _, identity := range lastSetup.Participants {
			_, exists := lookup[identity.NodeID]
			// add identity from previous epoch that is not in current epoch
			if !exists {
				otherEpochIdentities = append(otherEpochIdentities, identity)
			}
		}

	// during setup and committed phases (the end of the epoch) we include
	// identities that will join in the next epoch
	case flow.EpochPhaseSetup, flow.EpochPhaseCommitted:

		nextSetup, err := s.state.epoch.setups.ByID(status.NextEpoch.SetupID)
		if err != nil {
			return nil, fmt.Errorf("could not get next epoch setup: %w", err)
		}

		for _, identity := range nextSetup.Participants {
			_, exists := lookup[identity.NodeID]
			// add identity from next epoch that is not in current epoch
			if !exists {
				otherEpochIdentities = append(otherEpochIdentities, identity)
			}
		}

	default:
		return nil, fmt.Errorf("invalid epoch phase: %s", phase)
	}

	// add the identities from next/last epoch, with stake set to 0
	identities = append(
		identities,
		otherEpochIdentities.Map(mapfunc.WithStake(0))...,
	)

	// apply the filter to the participants
	identities = identities.Filter(selector)
	// apply a deterministic sort to the participants
	identities = identities.Order(order.ByNodeIDAsc)

	return identities, nil
}

func (s *Snapshot) Identity(nodeID flow.Identifier) (*flow.Identity, error) {
	// filter identities at snapshot for node ID
	identities, err := s.Identities(filter.HasNodeID(nodeID))
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}

	// check if node ID is part of identities
	if len(identities) == 0 {
		return nil, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return identities[0], nil
}

// Commit retrieves the latest execution state commitment at the current block snapshot. This
// commitment represents the execution state as currently finalized.
func (s *Snapshot) Commit() (flow.StateCommitment, error) {
	// get the ID of the sealed block
	seal, err := s.state.seals.ByBlockID(s.blockID)
	if err != nil {
		return flow.EmptyStateCommitment, fmt.Errorf("could not get look up sealed commit: %w", err)
	}
	return seal.FinalState, nil
}

func (s *Snapshot) Pending() ([]flow.Identifier, error) {
	return s.pending(s.blockID)
}

func (s *Snapshot) pending(blockID flow.Identifier) ([]flow.Identifier, error) {
	var pendingIDs []flow.Identifier
	err := s.state.db.View(procedure.LookupBlockChildren(blockID, &pendingIDs))
	if err != nil {
		return nil, fmt.Errorf("could not get pending children: %w", err)
	}

	for _, pendingID := range pendingIDs {
		additionalIDs, err := s.pending(pendingID)
		if err != nil {
			return nil, fmt.Errorf("could not get pending grandchildren: %w", err)
		}
		pendingIDs = append(pendingIDs, additionalIDs...)
	}
	return pendingIDs, nil
}

// Seed returns the random seed at the given indices for the current block snapshot.
func (s *Snapshot) Seed(indices ...uint32) ([]byte, error) {

	// get the current state snapshot head
	var childrenIDs []flow.Identifier
	err := s.state.db.View(procedure.LookupBlockChildren(s.blockID, &childrenIDs))
	if err != nil {
		return nil, fmt.Errorf("could not look up children: %w", err)
	}

	// check we have at least one child
	if len(childrenIDs) == 0 {
		return nil, state.NewNoValidChildBlockError("block doesn't have children yet")
	}

	// find the first child that has been validated
	var validChildID flow.Identifier
	for _, childID := range childrenIDs {
		var valid bool
		err = s.state.db.View(operation.RetrieveBlockValidity(childID, &valid))
		// skip blocks whose validity hasn't been checked yet
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not get child validity: %w", err)
		}
		if valid {
			validChildID = childID
			break
		}
	}

	if validChildID == flow.ZeroID {
		return nil, state.NewNoValidChildBlockError("block has no valid children")
	}

	// get the header of the first child (they all have the same threshold sig)
	head, err := s.state.headers.ByBlockID(validChildID)
	if err != nil {
		return nil, fmt.Errorf("could not get head: %w", err)
	}

	seed, err := seed.FromParentSignature(indices, head.ParentVoterSig)
	if err != nil {
		return nil, fmt.Errorf("could not create seed from header's signature: %w", err)
	}

	return seed, nil
}

func (s *Snapshot) Epochs() protocol.EpochQuery {
	return &EpochQuery{
		snap: s,
	}
}

// EpochQuery encapsulates querying epochs w.r.t. a snapshot.
type EpochQuery struct {
	snap *Snapshot
}

// Current returns the current epoch.
func (q *EpochQuery) Current() protocol.Epoch {

	status, err := q.snap.state.epoch.statuses.ByBlockID(q.snap.blockID)
	if err != nil {
		return NewInvalidEpoch(err)
	}
	setup, err := q.snap.state.epoch.setups.ByID(status.CurrentEpoch.SetupID)
	if err != nil {
		return NewInvalidEpoch(err)
	}
	commit, err := q.snap.state.epoch.commits.ByID(status.CurrentEpoch.CommitID)
	if err != nil {
		return NewInvalidEpoch(err)
	}

	return NewCommittedEpoch(setup, commit)
}

// Next returns the next epoch, if it is available.
func (q *EpochQuery) Next() protocol.Epoch {

	status, err := q.snap.state.epoch.statuses.ByBlockID(q.snap.blockID)
	if err != nil {
		return NewInvalidEpoch(err)
	}
	phase, err := status.Phase()
	if err != nil {
		return NewInvalidEpoch(err)
	}
	// if we are in the staking phase, the next epoch is not setup yet
	if phase == flow.EpochPhaseStaking {
		return NewInvalidEpoch(protocol.ErrNextEpochNotSetup)
	}

	// if we are in setup phase, return a SetupEpoch
	nextSetup, err := q.snap.state.epoch.setups.ByID(status.NextEpoch.SetupID)
	if err != nil {
		return NewInvalidEpoch(fmt.Errorf("failed to retrieve setup event for next epoch: %w", err))
	}
	if phase == flow.EpochPhaseSetup {
		return NewSetupEpoch(nextSetup)
	}

	// if we are in committed phase, return a CommittedEpoch
	nextCommit, err := q.snap.state.epoch.commits.ByID(status.NextEpoch.CommitID)
	if err != nil {
		return NewInvalidEpoch(fmt.Errorf("failed to retrieve commit event for next epoch: %w", err))
	}
	return NewCommittedEpoch(nextSetup, nextCommit)
}

// Previous returns the previous epoch. During the first epoch after the root
// block, this returns a sentinel error (since there is no previous epoch).
// For all other epochs, returns the previous epoch.
func (q *EpochQuery) Previous() protocol.Epoch {

	status, err := q.snap.state.epoch.statuses.ByBlockID(q.snap.blockID)
	if err != nil {
		return NewInvalidEpoch(err)
	}
	first, err := q.snap.state.headers.ByBlockID(status.FirstBlockID)
	if err != nil {
		return NewInvalidEpoch(err)
	}

	// CASE 1: we are in the first epoch after the root block, in which case
	// we return a sentinel error
	root, err := q.snap.state.Params().Root()
	if err != nil {
		return NewInvalidEpoch(err)
	}
	if first.Height == root.Height {
		return NewInvalidEpoch(protocol.ErrNoPreviousEpoch)
	}

	// CASE 2: we are in any other epoch, return the current epoch w.r.t. the
	// parent block of the first block in this epoch, which must be in the
	// previous epoch
	return q.snap.state.AtBlockID(first.ParentID).Epochs().Current()
}

// InvalidSnapshot represents a snapshot referencing an invalid block, or for
// which an error occurred while resolving the reference block.
type InvalidSnapshot struct {
	err error
}

func NewInvalidSnapshot(err error) *InvalidSnapshot {
	return &InvalidSnapshot{err: err}
}

func (u *InvalidSnapshot) Head() (*flow.Header, error) {
	return nil, u.err
}

func (u *InvalidSnapshot) Phase() (flow.EpochPhase, error) {
	return 0, u.err
}

func (u *InvalidSnapshot) Identities(_ flow.IdentityFilter) (flow.IdentityList, error) {
	return nil, u.err
}

func (u *InvalidSnapshot) Identity(_ flow.Identifier) (*flow.Identity, error) {
	return nil, u.err
}

func (u *InvalidSnapshot) Commit() (flow.StateCommitment, error) {
	return flow.EmptyStateCommitment, u.err
}

func (u *InvalidSnapshot) Pending() ([]flow.Identifier, error) {
	return nil, u.err
}

func (u *InvalidSnapshot) Seed(_ ...uint32) ([]byte, error) {
	return nil, u.err
}

// InvalidEpochQuery is an epoch query for an invalid snapshot.
type InvalidEpochQuery struct {
	err error
}

func (u *InvalidSnapshot) Epochs() protocol.EpochQuery {
	return &InvalidEpochQuery{err: u.err}
}

func (u *InvalidEpochQuery) Current() protocol.Epoch {
	return NewInvalidEpoch(u.err)
}

func (u *InvalidEpochQuery) Next() protocol.Epoch {
	return NewInvalidEpoch(u.err)
}

func (u *InvalidEpochQuery) Previous() protocol.Epoch {
	return NewInvalidEpoch(u.err)
}
