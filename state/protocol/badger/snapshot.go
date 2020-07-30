// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"errors"
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/model/epoch"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/flow/order"
	"github.com/dapperlabs/flow-go/state"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Snapshot represents a read-only immutable snapshot of the protocol state.
type Snapshot struct {
	err     error
	state   *State
	blockID flow.Identifier
}

// Identities retrieves all active ids at the given snapshot and
// applies the given filters.
func (s *Snapshot) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {
	if s.err != nil {
		return nil, s.err
	}

	// retrieve the current epoch
	var counter uint64
	err := s.state.db.View(operation.RetrieveEpochCounter(&counter))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root height: %w", err)
	}

	// retrieve the identities for the epoch
	var event epoch.Setup
	err = s.state.db.View(operation.RetrieveEpochSetup(counter, &event))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch identities: %w", err)
	}

	// TODO: We currently don't slash any nodes. However, once we receive
	// slashing events, we need a smart way to progressively store a growing
	// list of stake modifications per epoch, which should be applied here.

	// apply the filter to the identities
	identities := event.Participants.Filter(selector)

	// apply a deterministic sort to the participants
	sort.Slice(identities, func(i int, j int) bool {
		return order.ByNodeIDAsc(identities[i], identities[j])
	})

	return identities, err
}

func (s *Snapshot) Identity(nodeID flow.Identifier) (*flow.Identity, error) {
	if s.err != nil {
		return nil, s.err
	}

	// filter identities at snapshot for node ID
	identities, err := s.Identities(filter.HasNodeID(nodeID))
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}

	// check if node ID is part of identities
	if len(identities) == 0 {
		return nil, protocol.IdentityNotFoundErr{
			NodeID: nodeID,
		}
	}

	return identities[0], nil
}

func (s *Snapshot) Commit() (flow.StateCommitment, error) {
	if s.err != nil {
		return nil, s.err
	}

	// get the ID of the sealed block
	seal, err := s.state.seals.ByBlockID(s.blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get look up sealed commit: %w", err)
	}

	return seal.FinalState, nil
}

// Clusters sorts the list of node identities after filtering into the given
// number of clusters.
//
// This is guaranteed to be deterministic for an identical set of identities,
// regardless of the order.
func (s *Snapshot) Clusters() (flow.ClusterList, error) {
	if s.err != nil {
		return nil, s.err
	}

	// retrieve the epoch setup event for current epoch
	var counter uint64
	err := s.state.db.View(operation.RetrieveEpochCounter(&counter))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch counter: %w", err)
	}
	var setup epoch.Setup
	err = s.state.db.View(operation.RetrieveEpochSetup(counter, &setup))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch setup: %w", err)
	}

	// create the list of clusters
	clusters, err := flow.NewClusterList(
		setup.Assignments,
		setup.Participants.Filter(filter.HasRole(flow.RoleCollection)),
	)

	return clusters, nil
}

func (s *Snapshot) Head() (*flow.Header, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.state.headers.ByBlockID(s.blockID)
}

func (s *Snapshot) Seed(indices ...uint32) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}

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

func (s *Snapshot) Pending() ([]flow.Identifier, error) {
	if s.err != nil {
		return nil, s.err
	}
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

func (s *Snapshot) Epoch() (uint64, error) {
	if s.err != nil {
		return 0, s.err
	}

	// Retrieve the current header to get its view, as well as the current
	// epoch counter as a starting point.
	header, err := s.state.headers.ByBlockID(s.blockID)
	if err != nil {
		return 0, fmt.Errorf("could not retrieve snapshot header: %w", err)
	}
	var counter uint64
	err = s.state.db.View(operation.RetrieveEpochCounter(&counter))
	if err != nil {
		return 0, fmt.Errorf("could not retrieve epoch counter: %w", err)
	}

	// If the header's view is after the current epoch's view, we are dealing
	// with a header for the next epoch (it could be pending). We should never
	// have pending headers from two epochs in the future, so it's safe to
	// return here.
	var setup epoch.Setup
	err = s.state.db.View(operation.RetrieveEpochSetup(counter, &setup))
	if err != nil {
		return 0, fmt.Errorf("could not retrieve epoch setup: %w", err)
	}
	if header.View > setup.FinalView {
		return counter + 1, nil
	}

	// We can now iterate backwards through the epoch's as long as the header's
	// view is higher than the start of a given period. As soon as we find an
	// epoch that has a start lower than the current header's view, it means
	// the header falls in the epoch following that one.
	var start uint64
	for {

		// we have reached the first epoch, which this header has to be part of thus
		if counter == 0 {
			break
		}

		// get the start view of the epoch
		err = s.state.db.View(operation.LookupEpochStart(counter, &start))
		if err != nil {
			return 0, fmt.Errorf("could not look up epoch start (counter: %d): %w", counter, err)
		}

		// if start is bigger than the header view, it means the header is definitely not part
		// of the epoch; as the check still passed for the previous one, the header is thus
		// definitely part of the previously checked (next) period
		if start > header.View {
			counter++
			break
		}

		// the header still falls into the currently checked epoch; step back to the previous
		// one until we found one that the header doesn't belong to
		counter--
	}

	return counter, nil
}

func (s *Snapshot) DKG() protocol.DKG {
	return &DKG{snapshot: s}
}
