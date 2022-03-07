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
	"github.com/onflow/flow-go/state/fork"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/invalid"
	"github.com/onflow/flow-go/state/protocol/seed"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
)

// Snapshot implements the protocol.Snapshot interface.
// It represents a read-only immutable snapshot of the protocol state at the
// block it is constructed with. It allows efficient access to data associated directly
// with blocks at a given state (finalized, sealed), such as the related header, commit,
// seed or descending blocks. A block snapshot can lazily convert to an epoch snapshot in
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

// QuorumCertificate (QC) returns a valid quorum certificate pointing to the
// header at this snapshot. With the exception of the root block, a valid child
// block must be which contains the desired QC. The sentinel error
// state.NoValidChildBlockError is returned if the the QC is unknown.
//
// For root block snapshots, returns the root quorum certificate. For all other
// blocks, generates a quorum certificate from a valid child, if one exists.
func (s *Snapshot) QuorumCertificate() (*flow.QuorumCertificate, error) {

	// CASE 1: for the root block, return the root QC
	root, err := s.state.Params().Root()
	if err != nil {
		return nil, fmt.Errorf("could not get root: %w", err)
	}

	if s.blockID == root.ID() {
		var rootQC flow.QuorumCertificate
		err := s.state.db.View(operation.RetrieveRootQuorumCertificate(&rootQC))
		if err != nil {
			return nil, fmt.Errorf("could not retrieve root qc: %w", err)
		}
		return &rootQC, nil
	}

	// CASE 2: for any other block, generate the root QC from a valid child
	child, err := s.validChild()
	if err != nil {
		return nil, fmt.Errorf("could not get valid child of block %x: %w", s.blockID, err)
	}

	// sanity check: ensure the child has the snapshot block as parent
	if child.ParentID != s.blockID {
		return nil, fmt.Errorf("child parent id (%x) does not match snapshot id (%x)", child.ParentID, s.blockID)
	}

	// retrieve the full header as we need the view for the quorum certificate
	head, err := s.Head()
	if err != nil {
		return nil, fmt.Errorf("could not get head: %w", err)
	}

	qc := &flow.QuorumCertificate{
		View:      head.View,
		BlockID:   s.blockID,
		SignerIDs: child.ParentVoterIDs,
		SigData:   child.ParentVoterSigData,
	}

	return qc, nil
}

// validChild returns a child of the snapshot head that has been validated
// by HotStuff. Returns state.NoValidChildBlockError if no valid child exists.
//
// Any valid child may be returned. Subsequent calls are not guaranteed to
// return the same child.
func (s *Snapshot) validChild() (*flow.Header, error) {

	var childIDs []flow.Identifier
	err := s.state.db.View(procedure.LookupBlockChildren(s.blockID, &childIDs))
	if err != nil {
		return nil, fmt.Errorf("could not look up children: %w", err)
	}

	// find the first child that has been validated
	validChildID := flow.ZeroID
	for _, childID := range childIDs {
		var valid bool
		err = s.state.db.View(operation.RetrieveBlockValidity(childID, &valid))
		// skip blocks whose validity hasn't been checked yet
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to determine validity of child block %v: %w", childID, err)
		}
		if valid {
			validChildID = childID
			break
		}
	}

	if validChildID == flow.ZeroID {
		return nil, state.NewNoValidChildBlockErrorf("block has no valid children (total children: %d)", len(childIDs))
	}

	// get the header of the first child
	child, err := s.state.headers.ByBlockID(validChildID)
	return child, err
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

	// sort the identities so the 'Exists' binary search works
	identities := setup.Participants.Sort(order.ByNodeIDAsc)

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

		if !status.HasPrevious() {
			break
		}

		previousSetup, err := s.state.epoch.setups.ByID(status.PreviousEpoch.SetupID)
		if err != nil {
			return nil, fmt.Errorf("could not get previous epoch setup event: %w", err)
		}

		for _, identity := range previousSetup.Participants {
			exists := identities.Exists(identity)
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
			exists := identities.Exists(identity)

			// add identity from next epoch that is not in current epoch
			if !exists {
				otherEpochIdentities = append(otherEpochIdentities, identity)
			}
		}

	default:
		return nil, fmt.Errorf("invalid epoch phase: %s", phase)
	}

	// add the identities from next/last epoch, with weight set to 0
	identities = append(
		identities,
		otherEpochIdentities.Map(mapfunc.WithWeight(0))...,
	)

	// apply the filter to the participants
	identities = identities.Filter(selector)

	// apply a deterministic sort to the participants
	identities = identities.Sort(order.ByNodeIDAsc)

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
		return flow.DummyStateCommitment, fmt.Errorf("could not retrieve sealed state commit: %w", err)
	}
	return seal.FinalState, nil
}

func (s *Snapshot) SealedResult() (*flow.ExecutionResult, *flow.Seal, error) {
	seal, err := s.state.seals.ByBlockID(s.blockID)
	if err != nil {
		return nil, nil, fmt.Errorf("could not look up latest seal: %w", err)
	}
	result, err := s.state.results.ByID(seal.ResultID)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get latest result: %w", err)
	}
	return result, seal, nil
}

// SealingSegment will walk through the chain backward until we reach the block referenced
// by the latest seal and build a SealingSegment. As we visit each block we check each execution
// receipt in the block's payload to make sure we have a corresponding execution result, any execution
// results missing from blocks are stored in the SealingSegment.ExecutionResults field.
func (s *Snapshot) SealingSegment() (*flow.SealingSegment, error) {
	var rootHeight uint64
	err := s.state.db.View(operation.RetrieveRootHeight(&rootHeight))
	if err != nil {
		return nil, fmt.Errorf("could not get root height: %w", err)
	}
	head, err := s.Head()
	if err != nil {
		return nil, fmt.Errorf("could not get snapshot reference block: %w", err)
	}
	if head.Height < rootHeight {
		return nil, protocol.ErrSealingSegmentBelowRootBlock
	}

	seal, err := s.state.seals.ByBlockID(s.blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get seal for sealing segment: %w", err)
	}

	// walk through the chain backward until we reach the block referenced by
	// the latest seal - the returned segment includes this block
	builder := flow.NewSealingSegmentBuilder(s.state.results.ByID, s.state.seals.ByBlockID)
	scraper := func(header *flow.Header) error {
		blockID := header.ID()
		block, err := s.state.blocks.ByID(blockID)
		if err != nil {
			return fmt.Errorf("could not get block: %w", err)
		}

		err = builder.AddBlock(block)
		if err != nil {
			return fmt.Errorf("could not add block to sealing segment: %w", err)
		}

		return nil
	}

	err = fork.TraverseForward(s.state.headers, s.blockID, scraper, fork.IncludingBlock(seal.BlockID))
	if err != nil {
		return nil, fmt.Errorf("could not traverse sealing segment: %w", err)
	}

	segment, err := builder.SealingSegment()
	if err != nil {
		return nil, fmt.Errorf("could not build sealing segment: %w", err)
	}

	return segment, nil
}

func (s *Snapshot) Descendants() ([]flow.Identifier, error) {
	descendants, err := s.descendants(s.blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to traverse the descendants tree of block %v: %w", s.blockID, err)
	}
	return descendants, nil
}

func (s *Snapshot) ValidDescendants() ([]flow.Identifier, error) {
	valid, err := s.lookupValidity(s.blockID)
	if err != nil {
		return nil, fmt.Errorf("could not determine validity of block %v: %w", s.blockID, err)
	}
	if !valid {
		return []flow.Identifier{}, nil
	}

	descendants, err := s.validDescendants(s.blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to traverse the descendants tree of block %v: %w", s.blockID, err)
	}
	return descendants, nil
}

func (s *Snapshot) lookupChildren(blockID flow.Identifier) ([]flow.Identifier, error) {
	var children []flow.Identifier
	err := s.state.db.View(procedure.LookupBlockChildren(blockID, &children))
	if err != nil {
		return nil, fmt.Errorf("could not get children of block %v: %w", blockID, err)
	}
	return children, nil
}

func (s *Snapshot) lookupValidity(blockID flow.Identifier) (bool, error) {
	valid := false
	err := s.state.db.View(operation.RetrieveBlockValidity(blockID, &valid))
	if err != nil {
		// We only store the validity flag for blocks that have been marked valid.
		// For blocks that haven't been marked valid (yet), the flag is simply absent.
		if !errors.Is(err, storage.ErrNotFound) {
			return false, fmt.Errorf("could not retrieve validity of block %v: %w", blockID, err)
		}
	}
	return valid, nil
}

func (s *Snapshot) validDescendants(blockID flow.Identifier) ([]flow.Identifier, error) {
	var descendantIDs []flow.Identifier

	children, err := s.lookupChildren(blockID)
	if err != nil {
		return nil, err
	}

	for _, descendantID := range children {
		valid, err := s.lookupValidity(descendantID)
		if err != nil {
			return nil, err
		}

		if valid {
			descendantIDs = append(descendantIDs, descendantID)
			additionalIDs, err := s.validDescendants(descendantID)
			if err != nil {
				return nil, err
			}
			descendantIDs = append(descendantIDs, additionalIDs...)
		}
	}
	return descendantIDs, nil
}

func (s *Snapshot) descendants(blockID flow.Identifier) ([]flow.Identifier, error) {
	descendantIDs, err := s.lookupChildren(blockID)
	if err != nil {
		return nil, err
	}

	for _, descendantID := range descendantIDs {
		additionalIDs, err := s.descendants(descendantID)
		if err != nil {
			return nil, err
		}
		descendantIDs = append(descendantIDs, additionalIDs...)
	}
	return descendantIDs, nil
}

// RandomSource returns the seed for the current block snapshot.
// Expected error returns:
// * state.NoValidChildBlockError if no valid child is known
func (s *Snapshot) RandomSource() ([]byte, error) {

	// CASE 1: for the root block, generate the seed from the root qc
	root, err := s.state.Params().Root()
	if err != nil {
		return nil, fmt.Errorf("could not get root: %w", err)
	}

	if s.blockID == root.ID() {
		var rootQC flow.QuorumCertificate
		err := s.state.db.View(operation.RetrieveRootQuorumCertificate(&rootQC))
		if err != nil {
			return nil, fmt.Errorf("could not retrieve root qc: %w", err)
		}

		seed, err := seed.FromParentQCSignature(rootQC.SigData)
		if err != nil {
			return nil, fmt.Errorf("could not create seed from root qc: %w", err)
		}
		return seed, nil
	}

	// CASE 2: for any other block, use any valid child
	child, err := s.validChild()
	if err != nil {
		return nil, fmt.Errorf("failed to get valid child of block %x: %w", s.blockID, err)
	}

	seed, err := seed.FromParentQCSignature(child.ParentVoterSigData)
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

func (s *Snapshot) Params() protocol.GlobalParams {
	return s.state.Params()
}

// EpochQuery encapsulates querying epochs w.r.t. a snapshot.
type EpochQuery struct {
	snap *Snapshot
}

// Current returns the current epoch.
func (q *EpochQuery) Current() protocol.Epoch {

	status, err := q.snap.state.epoch.statuses.ByBlockID(q.snap.blockID)
	if err != nil {
		return invalid.NewEpoch(err)
	}
	setup, err := q.snap.state.epoch.setups.ByID(status.CurrentEpoch.SetupID)
	if err != nil {
		return invalid.NewEpoch(err)
	}
	commit, err := q.snap.state.epoch.commits.ByID(status.CurrentEpoch.CommitID)
	if err != nil {
		return invalid.NewEpoch(err)
	}

	epoch, err := inmem.NewCommittedEpoch(setup, commit)
	if err != nil {
		return invalid.NewEpoch(err)
	}
	return epoch
}

// Next returns the next epoch, if it is available.
func (q *EpochQuery) Next() protocol.Epoch {

	status, err := q.snap.state.epoch.statuses.ByBlockID(q.snap.blockID)
	if err != nil {
		return invalid.NewEpoch(err)
	}
	phase, err := status.Phase()
	if err != nil {
		return invalid.NewEpoch(err)
	}
	// if we are in the staking phase, the next epoch is not setup yet
	if phase == flow.EpochPhaseStaking {
		return invalid.NewEpoch(protocol.ErrNextEpochNotSetup)
	}

	// if we are in setup phase, return a SetupEpoch
	nextSetup, err := q.snap.state.epoch.setups.ByID(status.NextEpoch.SetupID)
	if err != nil {
		return invalid.NewEpoch(fmt.Errorf("failed to retrieve setup event for next epoch: %w", err))
	}
	if phase == flow.EpochPhaseSetup {
		epoch, err := inmem.NewSetupEpoch(nextSetup)
		if err != nil {
			return invalid.NewEpoch(err)
		}
		return epoch
	}

	// if we are in committed phase, return a CommittedEpoch
	nextCommit, err := q.snap.state.epoch.commits.ByID(status.NextEpoch.CommitID)
	if err != nil {
		return invalid.NewEpoch(fmt.Errorf("failed to retrieve commit event for next epoch: %w", err))
	}
	epoch, err := inmem.NewCommittedEpoch(nextSetup, nextCommit)
	if err != nil {
		return invalid.NewEpoch(err)
	}
	return epoch
}

// Previous returns the previous epoch. During the first epoch after the root
// block, this returns a sentinel error (since there is no previous epoch).
// For all other epochs, returns the previous epoch.
func (q *EpochQuery) Previous() protocol.Epoch {

	status, err := q.snap.state.epoch.statuses.ByBlockID(q.snap.blockID)
	if err != nil {
		return invalid.NewEpoch(err)
	}

	// CASE 1: there is no previous epoch - this indicates we are in the first
	// epoch after a spork root or genesis block
	if !status.HasPrevious() {
		return invalid.NewEpoch(protocol.ErrNoPreviousEpoch)
	}

	// CASE 2: we are in any other epoch - retrieve the setup and commit events
	// for the previous epoch
	setup, err := q.snap.state.epoch.setups.ByID(status.PreviousEpoch.SetupID)
	if err != nil {
		return invalid.NewEpoch(err)
	}
	commit, err := q.snap.state.epoch.commits.ByID(status.PreviousEpoch.CommitID)
	if err != nil {
		return invalid.NewEpoch(err)
	}

	epoch, err := inmem.NewCommittedEpoch(setup, commit)
	if err != nil {
		return invalid.NewEpoch(err)
	}
	return epoch
}
