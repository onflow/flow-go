// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/mapfunc"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/state/fork"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/invalid"
	"github.com/onflow/flow-go/state/protocol/seed"
	"github.com/onflow/flow-go/storage"
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

var _ protocol.Snapshot = (*Snapshot)(nil)

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
// header at this snapshot.
// The sentinel error storage.ErrNotFound is returned if the QC is unknown.
func (s *Snapshot) QuorumCertificate() (*flow.QuorumCertificate, error) {
	qc, err := s.state.qcs.ByBlockID(s.blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve quorum certificate for (%x): %w", s.blockID, err)
	}
	return qc, nil
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
	identities := setup.Participants.Sort(order.Canonical)

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
	identities = identities.Sort(order.Canonical)

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
	seal, err := s.state.seals.HighestInFork(s.blockID)
	if err != nil {
		return flow.DummyStateCommitment, fmt.Errorf("could not retrieve sealed state commit: %w", err)
	}
	return seal.FinalState, nil
}

func (s *Snapshot) SealedResult() (*flow.ExecutionResult, *flow.Seal, error) {
	seal, err := s.state.seals.HighestInFork(s.blockID)
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
// receipt in the block's payload to make sure we have a corresponding execution result, any
// execution results missing from blocks are stored in the `SealingSegment.ExecutionResults` field.
// See `model/flow/sealing_segment.md` for detailed technical specification of the Sealing Segment
//
// Expected errors during normal operations:
//   - protocol.ErrSealingSegmentBelowRootBlock if sealing segment would stretch beyond the node's local history cut-off
//   - protocol.UnfinalizedSealingSegmentError if sealing segment would contain unfinalized blocks (including orphaned blocks)
func (s *Snapshot) SealingSegment() (*flow.SealingSegment, error) {
	// Lets denote the highest block in the sealing segment `head` (initialized below).
	// Based on the tech spec `flow/sealing_segment.md`, the Sealing Segment must contain contain
	//  enough history to satisfy _all_ of the following conditions:
	//   (i) The highest sealed block as of `head` needs to be included in the sealing segment.
	//       This is relevant if `head` does not contain any seals.
	//  (ii) All blocks that are sealed by `head`. This is relevant if head` contains _multiple_ seals.
	// (iii) The sealing segment should contain the history back to (including):
	//       limitHeight := max(head.Height - flow.DefaultTransactionExpiry, SporkRootBlockHeight)
	// Per convention, we include the blocks for (i) in the `SealingSegment.Blocks`, while the
	// additional blocks for (ii) and optionally (iii) are contained in as `SealingSegment.ExtraBlocks`.
	head, err := s.state.blocks.ByID(s.blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get snapshot's reference block: %w", err)
	}
	if head.Header.Height < s.state.rootHeight {
		return nil, protocol.ErrSealingSegmentBelowRootBlock
	}

	// Verify that head of sealing segment is finalized.
	finalizedBlockAtHeight, err := s.state.headers.BlockIDByHeight(head.Header.Height)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, protocol.NewUnfinalizedSealingSegmentErrorf("head of sealing segment at height %d is not finalized: %w", head.Header.Height, err)
		}
		return nil, fmt.Errorf("exception while retrieving finzalized bloc, by height: %w", err)
	}
	if finalizedBlockAtHeight != s.blockID { // comparison of fixed-length arrays
		return nil, protocol.NewUnfinalizedSealingSegmentErrorf("head of sealing segment is orphaned, finalized block at height %d is %x", head.Header.Height, finalizedBlockAtHeight)
	}

	// STEP (i): highest sealed block as of `head` must be included.
	seal, err := s.state.seals.HighestInFork(s.blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get seal for sealing segment: %w", err)
	}
	blockSealedAtHead, err := s.state.headers.ByBlockID(seal.BlockID)
	if err != nil {
		return nil, fmt.Errorf("could not get block: %w", err)
	}

	// walk through the chain backward until we reach the block referenced by
	// the latest seal - the returned segment includes this block
	builder := flow.NewSealingSegmentBuilder(s.state.results.ByID, s.state.seals.HighestInFork)
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

	// STEP (ii): extend history down to the lowest block, whose seal is included in `head`
	lowestSealedByHead := blockSealedAtHead
	for _, sealInHead := range head.Payload.Seals {
		h, e := s.state.headers.ByBlockID(sealInHead.BlockID)
		if e != nil {
			return nil, fmt.Errorf("could not get block (id=%x) for seal: %w", seal.BlockID, e) // storage.ErrNotFound or exception
		}
		if h.Height < lowestSealedByHead.Height {
			lowestSealedByHead = h
		}
	}

	// STEP (iii): extended history to allow checking for duplicated collections, i.e.
	// limitHeight = max(head.Height - flow.DefaultTransactionExpiry, SporkRootBlockHeight)
	limitHeight := s.state.sporkRootBlockHeight
	if head.Header.Height > s.state.sporkRootBlockHeight+flow.DefaultTransactionExpiry {
		limitHeight = head.Header.Height - flow.DefaultTransactionExpiry
	}

	// As we have to satisfy (ii) _and_ (iii), we have to take the longest history, i.e. the lowest height.
	if lowestSealedByHead.Height < limitHeight {
		limitHeight = lowestSealedByHead.Height
		if limitHeight < s.state.sporkRootBlockHeight { // sanity check; should never happen
			return nil, fmt.Errorf("unexpected internal error: calculated history-cutoff at height %d, which is lower than the spork's root height %d", limitHeight, s.state.sporkRootBlockHeight)
		}
	}
	if limitHeight < blockSealedAtHead.Height {
		// we need to include extra blocks in sealing segment
		extraBlocksScraper := func(header *flow.Header) error {
			blockID := header.ID()
			block, err := s.state.blocks.ByID(blockID)
			if err != nil {
				return fmt.Errorf("could not get block: %w", err)
			}

			err = builder.AddExtraBlock(block)
			if err != nil {
				return fmt.Errorf("could not add block to sealing segment: %w", err)
			}

			return nil
		}

		err = fork.TraverseBackward(s.state.headers, blockSealedAtHead.ParentID, extraBlocksScraper, fork.IncludingHeight(limitHeight))
		if err != nil {
			return nil, fmt.Errorf("could not traverse extra blocks for sealing segment: %w", err)
		}
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

func (s *Snapshot) lookupChildren(blockID flow.Identifier) ([]flow.Identifier, error) {
	var children flow.IdentifierList
	err := s.state.db.View(procedure.LookupBlockChildren(blockID, &children))
	if err != nil {
		return nil, fmt.Errorf("could not get children of block %v: %w", blockID, err)
	}
	return children, nil
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
// * storage.ErrNotFound is returned if the QC is unknown.
func (s *Snapshot) RandomSource() ([]byte, error) {
	qc, err := s.QuorumCertificate()
	if err != nil {
		return nil, err
	}
	randomSource, err := seed.FromParentQCSignature(qc.SigData)
	if err != nil {
		return nil, fmt.Errorf("could not create seed from QC's signature: %w", err)
	}
	return randomSource, nil
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

	// all errors returned from storage reads here are unexpected, because all
	// snapshots reside within a current epoch, which must be queriable
	status, err := q.snap.state.epoch.statuses.ByBlockID(q.snap.blockID)
	if err != nil {
		return invalid.NewEpochf("could not get epoch status for block %x: %w", q.snap.blockID, err)
	}
	setup, err := q.snap.state.epoch.setups.ByID(status.CurrentEpoch.SetupID)
	if err != nil {
		return invalid.NewEpochf("could not get current EpochSetup (id=%x) for block %x: %w", status.CurrentEpoch.SetupID, q.snap.blockID, err)
	}
	commit, err := q.snap.state.epoch.commits.ByID(status.CurrentEpoch.CommitID)
	if err != nil {
		return invalid.NewEpochf("could not get current EpochCommit (id=%x) for block %x: %w", status.CurrentEpoch.CommitID, q.snap.blockID, err)
	}

	epoch, err := inmem.NewCommittedEpoch(setup, commit)
	if err != nil {
		// all conversion errors are critical and indicate we have stored invalid epoch info - strip error type info
		return invalid.NewEpochf("could not convert current epoch at block %x: %s", q.snap.blockID, err.Error())
	}
	return epoch
}

// Next returns the next epoch, if it is available.
func (q *EpochQuery) Next() protocol.Epoch {

	status, err := q.snap.state.epoch.statuses.ByBlockID(q.snap.blockID)
	if err != nil {
		return invalid.NewEpochf("could not get epoch status for block %x: %w", q.snap.blockID, err)
	}
	phase, err := status.Phase()
	if err != nil {
		// critical error: malformed EpochStatus in storage
		return invalid.NewEpochf("read malformed EpochStatus from storage: %w", err)
	}
	// if we are in the staking phase, the next epoch is not setup yet
	if phase == flow.EpochPhaseStaking {
		return invalid.NewEpoch(protocol.ErrNextEpochNotSetup)
	}

	// if we are in setup phase, return a SetupEpoch
	nextSetup, err := q.snap.state.epoch.setups.ByID(status.NextEpoch.SetupID)
	if err != nil {
		// all errors are critical, because we must be able to retrieve EpochSetup when in setup phase
		return invalid.NewEpochf("could not get next EpochSetup (id=%x) for block %x: %w", status.NextEpoch.SetupID, q.snap.blockID, err)
	}
	if phase == flow.EpochPhaseSetup {
		epoch, err := inmem.NewSetupEpoch(nextSetup)
		if err != nil {
			// all conversion errors are critical and indicate we have stored invalid epoch info - strip error type info
			return invalid.NewEpochf("could not convert next (setup) epoch: %s", err.Error())
		}
		return epoch
	}

	// if we are in committed phase, return a CommittedEpoch
	nextCommit, err := q.snap.state.epoch.commits.ByID(status.NextEpoch.CommitID)
	if err != nil {
		// all errors are critical, because we must be able to retrieve EpochCommit when in committed phase
		return invalid.NewEpochf("could not get next EpochCommit (id=%x) for block %x: %w", status.NextEpoch.CommitID, q.snap.blockID, err)
	}
	epoch, err := inmem.NewCommittedEpoch(nextSetup, nextCommit)
	if err != nil {
		// all conversion errors are critical and indicate we have stored invalid epoch info - strip error type info
		return invalid.NewEpochf("could not convert next (committed) epoch: %s", err.Error())
	}
	return epoch
}

// Previous returns the previous epoch. During the first epoch after the root
// block, this returns a sentinel error (since there is no previous epoch).
// For all other epochs, returns the previous epoch.
func (q *EpochQuery) Previous() protocol.Epoch {

	status, err := q.snap.state.epoch.statuses.ByBlockID(q.snap.blockID)
	if err != nil {
		return invalid.NewEpochf("could not get epoch status for block %x: %w", q.snap.blockID, err)
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
		// all errors are critical, because we must be able to retrieve EpochSetup for previous epoch
		return invalid.NewEpochf("could not get previous EpochSetup (id=%x) for block %x: %w", status.PreviousEpoch.SetupID, q.snap.blockID, err)
	}
	commit, err := q.snap.state.epoch.commits.ByID(status.PreviousEpoch.CommitID)
	if err != nil {
		// all errors are critical, because we must be able to retrieve EpochCommit for previous epoch
		return invalid.NewEpochf("could not get current EpochCommit (id=%x) for block %x: %w", status.PreviousEpoch.CommitID, q.snap.blockID, err)
	}

	epoch, err := inmem.NewCommittedEpoch(setup, commit)
	if err != nil {
		// all conversion errors are critical and indicate we have stored invalid epoch info - strip error type info
		return invalid.NewEpochf("could not convert previous epoch: %s", err.Error())
	}
	return epoch
}
