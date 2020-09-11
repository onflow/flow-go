// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"errors"
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/flow/order"
	"github.com/dapperlabs/flow-go/state"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Snapshot implements the protocol.Snapshot interface.
// It represents a read-only immutable snapshot of the protocol state at the
// block it is constructed with. It allows efficient access to data associated directly
// with blocks at a given state (finalized, sealed), such as the related header, commit,
// seed or pending children. A block snapshot can lazily convert to an epoch snapshot in
// order to make data associated directly with epochs accessible through its API.
type Snapshot struct {
	state   *State
	err     error           // cache any errors on the snapshot
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
	status, err := s.state.epochStatuses.ByBlockID(s.blockID)
	return status.Phase(), err
}

func (s *Snapshot) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {

	status, err := s.state.epochStatuses.ByBlockID(s.blockID)
	if err != nil {
		return nil, err
	}

	setup, err := s.state.setups.ByID(status.CurrentEpoch.Setup)
	if err != nil {
		return nil, err
	}

	// TODO: CAUTION SHORTCUT
	// We report the initial identities as of the EpochSetup event here.
	// Only valid as long as we don't have slashing
	identities := setup.Participants.Filter(selector)

	// apply a deterministic sort to the participants
	sort.Slice(identities, func(i int, j int) bool {
		return order.ByNodeIDAsc(identities[i], identities[j])
	})

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
		return nil, protocol.IdentityNotFoundErr{NodeID: nodeID}
	}
	return identities[0], nil
}

// Commit retrieves the latest execution state commitment at the current block snapshot. This
// commitment represents the execution state as currently finalized.
func (s *Snapshot) Commit() (flow.StateCommitment, error) {
	// get the ID of the sealed block
	seal, err := s.state.seals.ByBlockID(s.blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get look up sealed commit: %w", err)
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

	seed, err := protocol.SeedFromParentSignature(indices, head.ParentVoterSig)
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

// EpochQuery simplifies querying epochs w.r.t. a snapshot.
type EpochQuery struct {
	snap *Snapshot
}

// Current returns the current epoch.
func (q *EpochQuery) Current() protocol.Epoch {
	status, err := q.snap.state.epochStatuses.ByBlockID(q.snap.blockID)
	if err != nil {
		return NewInvalidEpoch(err)
	}
	setup, err := q.snap.state.setups.ByID(status.CurrentEpoch.Setup)
	if err != nil {
		return NewInvalidEpoch(err)
	}
	commit, err := q.snap.state.commits.ByID(status.CurrentEpoch.Commit)
	if err != nil {
		return NewInvalidEpoch(err)
	}
	return NewCommittedEpoch(setup, commit)
}

// Next returns the next epoch.
func (q *EpochQuery) Next() protocol.Epoch {
	status, err := q.snap.state.epochStatuses.ByBlockID(q.snap.blockID)
	if err != nil {
		return NewInvalidEpoch(err)
	}
	setup, err := q.snap.state.setups.ByID(status.CurrentEpoch.Setup)
	if err != nil {
		return NewInvalidEpoch(err)
	}
	return q.ByCounter(setup.Counter + 1)
}

// ByCounter returns the epoch with the given counter.
func (q *EpochQuery) ByCounter(counter uint64) protocol.Epoch {

	// get the current setup/commit events
	status, err := q.snap.state.epochStatuses.ByBlockID(q.snap.blockID)
	if err != nil {
		return NewInvalidEpoch(err)
	}
	currentSetup, err := q.snap.state.setups.ByID(status.CurrentEpoch.Setup)
	if err != nil {
		return NewInvalidEpoch(err)
	}
	currentCommit, err := q.snap.state.commits.ByID(status.CurrentEpoch.Commit)
	if err != nil {
		return NewInvalidEpoch(err)
	}

	switch {
	case counter < currentSetup.Counter:
		// we currently only support snapshots of the current and next Epoch
		return NewInvalidEpoch(fmt.Errorf("past epoch"))
	case counter == currentSetup.Counter:
		return NewCommittedEpoch(currentSetup, currentCommit)
	case counter == currentSetup.Counter+1:
		if status.NextEpoch.Setup == flow.ZeroID {
			return NewInvalidEpoch(fmt.Errorf("epoch still undefined"))
		}
		nextSetup, err := q.snap.state.setups.ByID(status.NextEpoch.Setup)
		if err != nil {
			return NewInvalidEpoch(fmt.Errorf("failed to retrieve setup event for next epoch: %w", err))
		}

		if status.NextEpoch.Commit == flow.ZeroID {
			return NewSetupEpoch(nextSetup)
		}
		nextCommit, err := q.snap.state.commits.ByID(status.NextEpoch.Commit)
		if err != nil {
			return NewInvalidEpoch(fmt.Errorf("failed to retrieve commit event for next epoch: %w", err))
		}
		return NewCommittedEpoch(nextSetup, nextCommit)
	default:
		// we currently only support snapshots of the current and next Epoch
		return NewInvalidEpoch(fmt.Errorf("epoch too far in future"))
	}
}

// ****************************************

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

func (u *InvalidSnapshot) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {
	return nil, u.err
}

func (u *InvalidSnapshot) Identity(nodeID flow.Identifier) (*flow.Identity, error) {
	return nil, u.err
}

func (u *InvalidSnapshot) Commit() (flow.StateCommitment, error) {
	return nil, u.err
}

func (u *InvalidSnapshot) Pending() ([]flow.Identifier, error) {
	return nil, u.err
}

func (u *InvalidSnapshot) Seed(indices ...uint32) ([]byte, error) {
	return nil, u.err
}

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

func (u *InvalidEpochQuery) ByCounter(_ uint64) protocol.Epoch {
	return NewInvalidEpoch(u.err)
}
