package hotstuff

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type VoteAggregator struct {
	viewState             *ViewState
	voteValidator         *Validator
	sigAggregator         SigAggregator
	highestPrunedView     uint64
	pendingVotes          *PendingVotes                                // keeps track of votes whose blocks can not be found
	viewToBlockIDSet      map[uint64]map[flow.Identifier]struct{}      // for pruning
	viewToVoteID          map[uint64]map[flow.Identifier]*types.Vote   // for detecting double voting
	createdQC             map[flow.Identifier]*types.QuorumCertificate // keeps track of QCs that have been made for blocks
	blockIDToVotingStatus map[flow.Identifier]*VotingStatus            // keeps track of accumulated votes and stakes for blocks
	proposerVotes         map[flow.Identifier]*types.Vote              // holds the votes of block proposers, so we can avoid passing around proposals everywhere
}

func NewVoteAggregator(highestPrunedView uint64, viewState *ViewState, voteValidator *Validator, sigAggregator SigAggregator) *VoteAggregator {
	return &VoteAggregator{
		highestPrunedView:     highestPrunedView,
		viewState:             viewState,
		voteValidator:         voteValidator,
		sigAggregator:         sigAggregator,
		pendingVotes:          NewPendingVotes(),
		viewToBlockIDSet:      make(map[uint64]map[flow.Identifier]struct{}),
		viewToVoteID:          make(map[uint64]map[flow.Identifier]*types.Vote),
		createdQC:             make(map[flow.Identifier]*types.QuorumCertificate),
		blockIDToVotingStatus: make(map[flow.Identifier]*VotingStatus),
		proposerVotes:         make(map[flow.Identifier]*types.Vote),
	}
}

// StorePendingVote stores the vote as a pending vote assuming the caller has checked that the voting
// block is currently missing.
// Note: Validations on these pending votes will be postponed until the block has been received.
func (va *VoteAggregator) StorePendingVote(vote *types.Vote) error {
	// check if the vote is for a view that has already been pruned (and is thus stale)
	if vote.View <= va.highestPrunedView { // cannot store vote for already pruned view
		return types.StaleVoteError{Vote: vote, HighestPrunedView: va.highestPrunedView}
	}

	// process the vote
	va.pendingVotes.AddVote(vote)
	va.updateState(vote)

	return nil
}

// StoreVoteAndBuildQC stores the vote assuming the caller has checked that the voting block is incorporated,
// and returns a QC if there are votes with enough stakes.
// The VoteAggregator builds a QC as soon as the number of votes allow this.
// While subsequent votes (past the required threshold) are not included in the QC anymore,
// VoteAggregator ALWAYS returns the same QC as the one returned before.
func (va *VoteAggregator) StoreVoteAndBuildQC(vote *types.Vote, block *types.Block) (*types.QuorumCertificate, error) {
	// if the QC for the block has been created before, return the QC
	oldQC, built := va.createdQC[block.BlockID]
	if built {
		return oldQC, nil
	}

	// ignore stale votes
	if vote.View <= va.highestPrunedView { // cannot build QC for already pruned view
		return nil, types.StaleVoteError{Vote: vote, HighestPrunedView: va.highestPrunedView}
	}

	// validate the vote and adding it to the accumulated voting status
	votingStatus, err := va.validateAndStoreIncorporatedVote(vote, block)
	if err != nil {
		return nil, fmt.Errorf("could not store incorporated vote: %w", err)
	}

	// try to build the QC with existing votes
	newQC, err := va.tryBuildQC(votingStatus)
	if err != nil {
		return nil, fmt.Errorf("could not build QC: %w", err)
	}
	return newQC, nil
}

// StoreProposerVote stores the vote for a block that was proposed.
func (va *VoteAggregator) StoreProposerVote(vote *types.Vote) {
	va.proposerVotes[vote.BlockID] = vote
}

// BuildQCOnReceivedBlock will attempt to build a QC for the given block when there are votes
// with enough stakes.
// It assumes that the proposer's vote has been stored by calling StoreProposerVote
func (va *VoteAggregator) BuildQCOnReceivedBlock(block *types.Block) (*types.QuorumCertificate, error) {
	// return the QC that was built before if exists
	oldQC, built := va.createdQC[block.BlockID]
	if built {
		return oldQC, nil
	}

	if block.View <= va.highestPrunedView {
		// cannot build QC for already pruned view
		return nil, types.StaleBlockError{Block: block, HighestPrunedView: va.highestPrunedView}
	}

	proposerVote, ok := va.proposerVotes[block.BlockID]
	if !ok {
		return nil, fmt.Errorf("could not get proposer vote")
	}

	// accumulate leader vote first to ensure leader's vote is always included in the QC
	voteStatus, err := va.validateAndStoreIncorporatedVote(proposerVote, block)
	if err != nil {
		return nil, fmt.Errorf("leader vote is invalid %w", err)
	}

	// accumulate pending votes by order
	pendingStatus, exists := va.pendingVotes.votes[block.BlockID]
	if exists {
		va.convertPendingVotes(pendingStatus.orderedVotes, block)
	}

	// try building QC with existing valid votes
	qc, err := va.tryBuildQC(voteStatus)
	if err != nil {
		return nil, fmt.Errorf("could not build QC on receiving block: %w", err)
	}

	delete(va.proposerVotes, block.BlockID)
	return qc, nil
}

func (va *VoteAggregator) convertPendingVotes(pendingVotes []*types.Vote, block *types.Block) {
	for _, vote := range pendingVotes {
		voteStatus, err := va.validateAndStoreIncorporatedVote(vote, block)
		if err != nil {
			continue
		}
		// if threshold is reached, the rest of the votes can be ignored
		if voteStatus.CanBuildQC() {
			break
		}
	}
	delete(va.pendingVotes.votes, block.BlockID)
}

// PruneByView will delete all votes equal or below to the given view, as well as related indexes.
func (va *VoteAggregator) PruneByView(view uint64) {
	if view <= va.highestPrunedView {
		return
	}
	for i := va.highestPrunedView + 1; i <= view; i++ {
		blockIDStrSet := va.viewToBlockIDSet[i]
		for blockID := range blockIDStrSet {
			delete(va.pendingVotes.votes, blockID)
			delete(va.blockIDToVotingStatus, blockID)
			delete(va.createdQC, blockID)
			delete(va.proposerVotes, blockID)
		}
		delete(va.viewToBlockIDSet, i)
		delete(va.viewToVoteID, i)
	}
	va.highestPrunedView = view
}

// storeIncorporatedVote stores incorporated votes and accumulate stakes
// it drops invalid votes and duplicate votes
func (va *VoteAggregator) validateAndStoreIncorporatedVote(vote *types.Vote, block *types.Block) (*VotingStatus, error) {
	voter, err := va.voteValidator.ValidateVote(vote, block)
	if err != nil {
		return nil, fmt.Errorf("could not validate incorporated vote: %w", err)
	}
	err = va.checkDoubleVote(vote, voter)
	if err != nil {
		return nil, fmt.Errorf("could not store vote: %w", err)
	}
	// update existing voting status or create a new one
	votingStatus, exists := va.blockIDToVotingStatus[vote.BlockID]
	if !exists {
		threshold, err := va.viewState.GetQCStakeThresholdAtBlock(vote.BlockID)
		if err != nil {
			return nil, fmt.Errorf("could not get stake threshold: %w", err)
		}
		identities, err := va.viewState.GetStakedIdentitiesAtBlock(vote.BlockID)
		if err != nil {
			return nil, fmt.Errorf("could not get identities: %w", err)
		}
		votingStatus = NewVotingStatus(va.sigAggregator, threshold, vote.View, uint32(len(identities)), voter, vote.BlockID)
		va.blockIDToVotingStatus[vote.BlockID] = votingStatus
	}
	votingStatus.AddVote(vote, voter)
	va.updateState(vote)
	return votingStatus, nil
}

func (va *VoteAggregator) updateState(vote *types.Vote) {
	voterID := vote.Signature.SignerID

	// update viewToVoteID
	idToVote, exists := va.viewToVoteID[vote.View]
	if exists {
		idToVote[voterID] = vote
	} else {
		idToVote = make(map[flow.Identifier]*types.Vote)
		idToVote[voterID] = vote
		va.viewToVoteID[vote.View] = idToVote
	}

	// update viewToBlockIDSet
	blockIDSet, exists := va.viewToBlockIDSet[vote.View]
	if exists {
		blockIDSet[vote.BlockID] = struct{}{}
	} else {
		blockIDSet = make(map[flow.Identifier]struct{})
		blockIDSet[vote.BlockID] = struct{}{}
		va.viewToBlockIDSet[vote.View] = blockIDSet
	}
}

func (va *VoteAggregator) tryBuildQC(votingStatus *VotingStatus) (*types.QuorumCertificate, error) {
	qc, err := votingStatus.TryBuildQC()
	if err != nil {
		return nil, err
	}

	va.createdQC[votingStatus.blockID] = qc
	return qc, nil
}

// double voting is detected when the voter has voted a different block at the same view before
func (va *VoteAggregator) checkDoubleVote(vote *types.Vote, sender *flow.Identity) error {
	idToVotes, ok := va.viewToVoteID[vote.View]
	if !ok {
		// never voted by anyone
		return nil
	}
	originalVote, exists := idToVotes[sender.ID()]
	if !exists {
		// never voted by this sender
		return nil
	}
	if originalVote.BlockID == vote.BlockID {
		// voted and is the same vote as the vote received before
		return nil
	}
	return types.DoubleVoteError{
		OriginalVote: originalVote,
		DoubleVote:   vote,
	}
}
