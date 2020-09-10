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
	state       *State
	header      *flow.Header
	setupEvent  *flow.EpochSetup
	commitEvent *flow.EpochCommit
}

func NewSnapshot(state *State, header *flow.Header, setupEvent *flow.EpochSetup, commitEvent *flow.EpochCommit) *Snapshot {
	return &Snapshot{
		state:       state,
		header:      header,
		setupEvent:  setupEvent,
		commitEvent: commitEvent,
	}
}

func (s *Snapshot) Head() (*flow.Header, error) {
	return s.header, nil
}

func (s *Snapshot) Epoch() (uint64, error) {
	return s.setupEvent.Counter, nil
}

func (s *Snapshot) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {
	// TODO: CAUTION SHORTCUT
	// We report the initial identities as of the EpochSetup event here.
	// Only valid as long as we don't have slashing
	identities := s.setupEvent.Participants.Filter(selector)

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
	seal, err := s.state.seals.ByBlockID(s.header.ID())
	if err != nil {
		return nil, fmt.Errorf("could not get look up sealed commit: %w", err)
	}
	return seal.FinalState, nil
}

func (s *Snapshot) Pending() ([]flow.Identifier, error) {
	return s.pending(s.header.ID())
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
func (s *Snapshot) RandomBeaconSeed(indices ...uint32) ([]byte, error) {

	// get the current state snapshot head
	var childrenIDs []flow.Identifier
	err := s.state.db.View(procedure.LookupBlockChildren(s.header.ID(), &childrenIDs))
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

func (s *Snapshot) EpochSnapshot(counter uint64) protocol.EpochSnapshot {
	switch {
	case counter < s.setupEvent.Counter:
		// we currently only support snapshots of the current and next Epoch
		return NewUndefinedEpochSnapshot(fmt.Errorf("past epoch"))
	case counter == s.setupEvent.Counter:
		return NewEpochCommitSnapshot(s.header, s.setupEvent, s.commitEvent)
	case counter == s.setupEvent.Counter+1:
		epochState, err := s.state.epochStates.ByBlockID(s.header.ID())
		if err != nil {
			return NewUndefinedEpochSnapshot(fmt.Errorf("failed to retrieve epoch state for head: %w", err))
		}

		if epochState.NextEpoch.SetupEventID == flow.ZeroID {
			return NewUndefinedEpochSnapshot(fmt.Errorf("epoch still undefined"))
		}
		setupEvent, err := s.state.setups.BySetupID(epochState.NextEpoch.SetupEventID)
		if err != nil {
			return NewUndefinedEpochSnapshot(fmt.Errorf("failed to retrieve setup event for epoch: %w", err))
		}

		if epochState.NextEpoch.CommitEventID == flow.ZeroID {
			return NewEpochSetupSnapshot(s.header, setupEvent)
		}
		commitEvent, err := s.state.commits.ByCommitID(epochState.NextEpoch.CommitEventID)
		if err != nil {
			return NewUndefinedEpochSnapshot(fmt.Errorf("failed to retrieve commit event for epoch: %w", err))
		}
		return NewEpochCommitSnapshot(s.header, setupEvent, commitEvent)
	default:
		// we currently only support snapshots of the current and next Epoch
		return NewUndefinedEpochSnapshot(fmt.Errorf("epoch too far in future"))
	}
}

// ****************************************

type UndefinedSnapshot struct {
	err error
}

func NewUndefinedSnapshot(err error) *UndefinedSnapshot {
	return &UndefinedSnapshot{err: err}
}

func (u *UndefinedSnapshot) Head() (*flow.Header, error) {
	return nil, u.err
}

func (u *UndefinedSnapshot) Epoch() (uint64, error) {
	return 0, u.err
}

func (u *UndefinedSnapshot) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {
	return nil, u.err
}

func (u *UndefinedSnapshot) Identity(nodeID flow.Identifier) (*flow.Identity, error) {
	return nil, u.err
}

func (u *UndefinedSnapshot) Commit() (flow.StateCommitment, error) {
	return nil, u.err
}

func (u *UndefinedSnapshot) Pending() ([]flow.Identifier, error) {
	return nil, u.err
}

func (u *UndefinedSnapshot) RandomBeaconSeed(indices ...uint32) ([]byte, error) {
	return nil, u.err
}

func (u *UndefinedSnapshot) EpochSnapshot(counter uint64) protocol.EpochSnapshot {
	return NewUndefinedEpochSnapshot(u.err)
}
