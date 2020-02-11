package hotstuff

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type VoteAggregator struct {
	viewState      ViewState
	voteValidator  *Validator
	lastPrunedView uint64
	// keeps track of votes whose blocks can not be found
	pendingVotes *PendingVotes
	// For pruning
	viewToBlockIDSet map[uint64]map[flow.Identifier]struct{}
	// For detecting double voting
	viewToIDToVote map[uint64]map[flow.Identifier]*types.Vote
	// keeps track of QCs that have been made for blocks
	createdQC map[flow.Identifier]*types.QuorumCertificate
	// keeps track of accumulated votes and stakes for blocks
	blockHashToVotingStatus map[flow.Identifier]*VotingStatus
}

func NewVoteAggregator(lastPruneView uint64, viewState ViewState, voteValidator *Validator) *VoteAggregator {
	return &VoteAggregator{
		lastPrunedView:          lastPruneView,
		viewState:               viewState,
		voteValidator:           voteValidator,
		pendingVotes:            NewPendingVotes(),
		viewToBlockIDSet:        make(map[uint64]map[flow.Identifier]struct{}),
		viewToIDToVote:          make(map[uint64]map[flow.Identifier]*types.Vote),
		blockHashToVotingStatus: make(map[flow.Identifier]*VotingStatus),
		createdQC:               make(map[flow.Identifier]*types.QuorumCertificate),
	}
}

// StorePendingVote stores the vote as a pending vote assuming the caller has checked that the voting
// block is currently missing.
// Note: Validations on these pending votes will be postponed until the block has been received.
func (va *VoteAggregator) StorePendingVote(vote *types.Vote) error {
	if vote.View <= va.lastPrunedView {
		return fmt.Errorf("could not store pending vote: %w", types.StaleVoteError{
			Vote:          vote,
			FinalizedView: va.lastPrunedView,
		})
	}
	voter, err := va.voteValidator.ValidateVote(vote, nil)
	if err != nil {
		return fmt.Errorf("could not store pending vote: %w", err)
	}
	err = va.checkDoubleVote(vote, voter)
	if err != nil {
		return fmt.Errorf("could not store pending vote: %w", err)
	}
	// have verified pending vote
	va.pendingVotes.AddVote(vote)
	va.updateState(vote, voter)
	return nil
}

// StoreVoteAndBuildQC stores the vote assuming the caller has checked that the voting block is incorporated,
// and returns a QC if there are votes with enough stakes.
// The VoteAggregator builds a QC as soon as the number of votes allow this.
// While subsequent votes (past the required threshold) are not included in the QC anymore,
// VoteAggregator ALWAYS returns the same QC as the one returned before.
func (va *VoteAggregator) StoreVoteAndBuildQC(vote *types.Vote, bp *types.BlockProposal) (*types.QuorumCertificate, error) {
	// if the QC for the block has been created before, return the QC
	oldQC, built := va.createdQC[bp.BlockID()]
	if built {
		return oldQC, nil
	}
	// ignore stale votes
	if vote.View <= va.lastPrunedView {
		return nil, fmt.Errorf("could not store incorporated vote: %w", types.StaleVoteError{
			Vote:          vote,
			FinalizedView: va.lastPrunedView,
		})
	}
	votingStatus, err := va.validateAndStoreIncorporatedVote(vote, bp)
	if err != nil {
		return nil, fmt.Errorf("could not store incorporated vote: %w", err)
	}
	newQC, err := va.tryBuildQC(votingStatus)
	if err != nil {
		return nil, fmt.Errorf("could not build QC: %w", err)
	}
	return newQC, nil
}

// BuildQCForBlockProposal will extract a leader vote out of the block proposal and
// attempt to build a QC for the given block proposal when there are votes
// with enough stakes.
func (va *VoteAggregator) BuildQCOnReceivingBlock(bp *types.BlockProposal) (*types.QuorumCertificate, error) {
	oldQC, built := va.createdQC[bp.BlockID()]
	if built {
		return oldQC, nil
	}
	if bp.View() <= va.lastPrunedView {
		return nil, fmt.Errorf("could not build QC on receiving block: %w", types.StaleBlockError{BlockProposal: bp, FinalizedView: va.lastPrunedView})
	}
	// accumulate leader vote first
	leaderVote := bp.ProposersVote()
	voteStatus, err := va.validateAndStoreIncorporatedVote(leaderVote, bp)
	if err != nil {
		return nil, fmt.Errorf("leader vote is invalid %w", err)
	}
	// accumulate pending votes by order
	pendingStatus, exists := va.pendingVotes.votes[bp.BlockID()]
	if exists {
		va.convertPendingVotes(pendingStatus.orderedVotes, bp)
	}
	qc, err := va.tryBuildQC(voteStatus)
	if err != nil {
		return nil, fmt.Errorf("could not build QC on receiving block: %w", err)
	}
	return qc, nil
}

func (va *VoteAggregator) convertPendingVotes(pendingVotes []*types.Vote, bp *types.BlockProposal) {
	for _, vote := range pendingVotes {
		voteStatus, err := va.validateAndStoreIncorporatedVote(vote, bp)
		if err != nil {
			continue
		}
		// if threshold is reached, the rest of the votes can be ignored
		if voteStatus.CanBuildQC() {
			break
		}
	}
	delete(va.pendingVotes.votes, bp.BlockID())
}

// PruneByView will delete all votes equal or below to the given view, as well as related indexes.
func (va *VoteAggregator) PruneByView(view uint64) {
	if view <= va.lastPrunedView {
		return
	}
	for i := va.lastPrunedView + 1; i <= view; i++ {
		blockIDStrSet := va.viewToBlockIDSet[i]
		for blockID := range blockIDStrSet {
			delete(va.pendingVotes.votes, blockID)
			delete(va.blockHashToVotingStatus, blockID)
			delete(va.createdQC, blockID)
		}
		delete(va.viewToBlockIDSet, i)
		delete(va.viewToIDToVote, i)
	}
	va.lastPrunedView = view
}

// storeIncorporatedVote stores incorporated votes and accumulate stakes
// it drops invalid votes and duplicate votes
func (va *VoteAggregator) validateAndStoreIncorporatedVote(vote *types.Vote, bp *types.BlockProposal) (*VotingStatus, error) {
	voter, err := va.voteValidator.ValidateVote(vote, bp)
	if err != nil {
		return nil, fmt.Errorf("could not validate incorporated vote: %w", err)
	}
	err = va.checkDoubleVote(vote, voter)
	if err != nil {
		return nil, fmt.Errorf("could not store vote: %w", err)
	}
	// update existing voting status or create a new one
	votingStatus, exists := va.blockHashToVotingStatus[vote.BlockID]
	if !exists {
		threshold, err := va.viewState.GetQCStakeThresholdForBlockID(vote.BlockID)
		if err != nil {
			return nil, fmt.Errorf("could not get stake threshold: %w", err)
		}
		identities, err := va.viewState.GetIdentitiesForBlockID(vote.BlockID)
		if err != nil {
			return nil, fmt.Errorf("could not get identities: %w", err)
		}
		votingStatus = NewVotingStatus(threshold, vote.View, uint32(len(identities)), voter, vote.BlockID)
		va.blockHashToVotingStatus[vote.BlockID] = votingStatus
	}
	votingStatus.AddVote(vote, voter)
	va.updateState(vote, voter)
	return votingStatus, nil
}

func (va *VoteAggregator) updateState(vote *types.Vote, voter *flow.Identity) {
	idToVote, exists := va.viewToIDToVote[vote.View]
	if exists {
		idToVote[voter.ID()] = vote
	} else {
		va.viewToIDToVote[vote.View] = map[flow.Identifier]*types.Vote{}
		va.viewToIDToVote[vote.View][voter.ID()] = vote
	}
	blockIDSet, exists := va.viewToBlockIDSet[vote.View]
	if exists {
		blockIDSet[vote.BlockID] = struct{}{}
	} else {
		va.viewToBlockIDSet[vote.View] = make(map[flow.Identifier]struct{})
		va.viewToBlockIDSet[vote.View][vote.BlockID] = struct{}{}
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
	idToVotes, ok := va.viewToIDToVote[vote.View]
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
