package badger

import (
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/fork"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/invalid"
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

// FinalizedSnapshot represents a read-only immutable snapshot of the protocol state
// at a finalized block. It is guaranteed to have a header available.
type FinalizedSnapshot struct {
	Snapshot
	header *flow.Header
}

var _ protocol.Snapshot = (*Snapshot)(nil)
var _ protocol.Snapshot = (*FinalizedSnapshot)(nil)

// newSnapshotWithIncorporatedReferenceBlock creates a new state snapshot with the given reference block.
// CAUTION: The caller is responsible for ensuring that the reference block has been incorporated.
func newSnapshotWithIncorporatedReferenceBlock(state *State, blockID flow.Identifier) *Snapshot {
	return &Snapshot{
		state:   state,
		blockID: blockID,
	}
}

// NewFinalizedSnapshot instantiates a `FinalizedSnapshot`.
// CAUTION: the header's ID _must_ match `blockID` (not checked)
func NewFinalizedSnapshot(state *State, blockID flow.Identifier, header *flow.Header) *FinalizedSnapshot {
	return &FinalizedSnapshot{
		Snapshot: Snapshot{
			state:   state,
			blockID: blockID,
		},
		header: header,
	}
}

func (s *FinalizedSnapshot) Head() (*flow.Header, error) {
	return s.header, nil
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
	psSnapshot, err := s.state.protocolState.AtBlockID(s.blockID)
	if err != nil {
		return flow.EpochPhaseUndefined, fmt.Errorf("could not retrieve protocol state snapshot: %w", err)
	}
	return psSnapshot.EpochPhase(), nil
}

func (s *Snapshot) Identities(selector flow.IdentityFilter[flow.Identity]) (flow.IdentityList, error) {
	psSnapshot, err := s.state.protocolState.AtBlockID(s.blockID)
	if err != nil {
		return nil, err
	}

	// apply the filter to the participants
	identities := psSnapshot.Identities().Filter(selector)
	return identities, nil
}

func (s *Snapshot) Identity(nodeID flow.Identifier) (*flow.Identity, error) {
	// filter identities at snapshot for node ID
	identities, err := s.Identities(filter.HasNodeID[flow.Identity](nodeID))
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
	if head.Header.Height < s.state.finalizedRootHeight {
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

// RandomSource returns the seed for the current block's snapshot.
// Expected error returns:
// * storage.ErrNotFound is returned if the QC is unknown.
func (s *Snapshot) RandomSource() ([]byte, error) {
	qc, err := s.QuorumCertificate()
	if err != nil {
		return nil, err
	}
	randomSource, err := model.BeaconSignature(qc)
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

// ProtocolState returns the dynamic protocol state that the Head block commits to. The
// compliance layer guarantees that only valid blocks are appended to the protocol state.
// For each block stored there should be a protocol state stored.
func (s *Snapshot) ProtocolState() (protocol.DynamicProtocolState, error) {
	return s.state.protocolState.AtBlockID(s.blockID)
}

func (s *Snapshot) VersionBeacon() (*flow.SealedVersionBeacon, error) {
	head, err := s.state.headers.ByBlockID(s.blockID)
	if err != nil {
		return nil, err
	}

	return s.state.versionBeacons.Highest(head.Height)
}

// EpochQuery encapsulates querying epochs w.r.t. a snapshot.
type EpochQuery struct {
	snap *Snapshot
}

// Current returns the current epoch.
func (q *EpochQuery) Current() protocol.Epoch {
	// all errors returned from storage reads here are unexpected, because all
	// snapshots reside within a current epoch, which must be queryable
	psSnapshot, err := q.snap.state.protocolState.AtBlockID(q.snap.blockID)
	if err != nil {
		return invalid.NewEpochf("could not get protocol state snapshot at block %x: %w", q.snap.blockID, err)
	}

	setup := psSnapshot.EpochSetup()
	commit := psSnapshot.EpochCommit()
	firstHeight, _, epochStarted, _, err := q.retrieveEpochHeightBounds(setup.Counter)
	if err != nil {
		return invalid.NewEpochf("could not get current epoch height bounds: %s", err.Error())
	}
	if epochStarted {
		return inmem.NewStartedEpoch(setup, commit, firstHeight)
	}
	return inmem.NewCommittedEpoch(setup, commit)
}

// Next returns the next epoch, if it is available.
func (q *EpochQuery) Next() protocol.Epoch {

	psSnapshot, err := q.snap.state.protocolState.AtBlockID(q.snap.blockID)
	if err != nil {
		return invalid.NewEpochf("could not get protocol state snapshot at block %x: %w", q.snap.blockID, err)
	}
	phase := psSnapshot.EpochPhase()
	// if we are in the staking phase, the next epoch is not setup yet
	if phase == flow.EpochPhaseStaking {
		return invalid.NewEpoch(protocol.ErrNextEpochNotSetup)
	}
	// if we are in setup phase, return a SetupEpoch
	nextSetup := psSnapshot.Entry().NextEpochSetup
	if phase == flow.EpochPhaseSetup {
		return inmem.NewSetupEpoch(nextSetup)
	}
	// if we are in committed phase, return a CommittedEpoch
	nextCommit := psSnapshot.Entry().NextEpochCommit
	if phase == flow.EpochPhaseCommitted {
		return inmem.NewCommittedEpoch(nextSetup, nextCommit)
	}
	return invalid.NewEpochf("data corruption: unknown epoch phase implies malformed protocol state epoch data")
}

// Previous returns the previous epoch. During the first epoch after the root
// block, this returns a sentinel error (since there is no previous epoch).
// For all other epochs, returns the previous epoch.
func (q *EpochQuery) Previous() protocol.Epoch {

	psSnapshot, err := q.snap.state.protocolState.AtBlockID(q.snap.blockID)
	if err != nil {
		return invalid.NewEpochf("could not get protocol state snapshot at block %x: %w", q.snap.blockID, err)
	}
	entry := psSnapshot.Entry()

	// CASE 1: there is no previous epoch - this indicates we are in the first
	// epoch after a spork root or genesis block
	if !psSnapshot.PreviousEpochExists() {
		return invalid.NewEpoch(protocol.ErrNoPreviousEpoch)
	}

	// CASE 2: we are in any other epoch - retrieve the setup and commit events
	// for the previous epoch
	setup := entry.PreviousEpochSetup
	commit := entry.PreviousEpochCommit

	firstHeight, finalHeight, _, epochEnded, err := q.retrieveEpochHeightBounds(setup.Counter)
	if err != nil {
		return invalid.NewEpochf("could not get epoch height bounds: %w", err)
	}
	if epochEnded {
		return inmem.NewEndedEpoch(setup, commit, firstHeight, finalHeight)
	}
	return inmem.NewStartedEpoch(setup, commit, firstHeight)
}

// retrieveEpochHeightBounds retrieves the height bounds for an epoch.
// Height bounds are NOT fork-aware, and are only determined upon finalization.
//
// Since the protocol state's API is fork-aware, we may be querying an
// un-finalized block, as in the following example:
//
//	Epoch 1    Epoch 2
//	A <- B <-|- C <- D
//
// Suppose block B is the latest finalized block and we have queried block D.
// Then, the transition from epoch 1 to 2 has not been committed, because the first block of epoch 2 has not been finalized.
// In this case, the final block of Epoch 1, from the perspective of block D, is unknown.
// There are edge-case scenarios, where a different fork could exist (as illustrated below)
// that still adds additional blocks to Epoch 1.
//
//	Epoch 1      Epoch 2
//	A <- B <---|-- C <- D
//	     ^
//	     ╰ X <-|- X <- Y <- Z
//
// Returns:
//   - (0, 0, false, false, nil) if epoch is not started
//   - (firstHeight, 0, true, false, nil) if epoch is started but not ended
//   - (firstHeight, finalHeight, true, true, nil) if epoch is ended
//
// No errors are expected during normal operation.
func (q *EpochQuery) retrieveEpochHeightBounds(epoch uint64) (firstHeight, finalHeight uint64, isFirstBlockFinalized, isLastBlockFinalized bool, err error) {
	err = q.snap.state.db.View(func(tx *badger.Txn) error {
		// Retrieve the epoch's first height
		err = operation.RetrieveEpochFirstHeight(epoch, &firstHeight)(tx)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				isFirstBlockFinalized = false
				isLastBlockFinalized = false
				return nil
			}
			return err // unexpected error
		}
		isFirstBlockFinalized = true

		var subsequentEpochFirstHeight uint64
		err = operation.RetrieveEpochFirstHeight(epoch+1, &subsequentEpochFirstHeight)(tx)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				isLastBlockFinalized = false
				return nil
			}
			return err // unexpected error
		}
		finalHeight = subsequentEpochFirstHeight - 1
		isLastBlockFinalized = true

		return nil
	})
	if err != nil {
		return 0, 0, false, false, err
	}
	return firstHeight, finalHeight, isFirstBlockFinalized, isLastBlockFinalized, nil
}
