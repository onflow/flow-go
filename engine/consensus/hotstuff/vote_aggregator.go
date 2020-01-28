package hotstuff

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type VoteAggregator struct {
	log            zerolog.Logger
	viewState      *ViewState
	voteValidator  *Validator
	lastPrunedView uint64
	// For pruning
	viewToBlockMRH map[uint64][][]byte
	// keeps track of votes whose blocks can not be found
	pendingVotes map[string]map[string]*types.Vote
	// keeps track of QCs that have been made for blocks
	createdQC map[string]*types.QuorumCertificate
	// keeps track of accumulated votes and stakes for blocks
	blockHashToVotingStatus map[string]*VotingStatus
	// For detecting double voting
	viewToIDToVote map[uint64]map[flow.Identifier]*types.Vote
}

func NewVoteAggregator(log zerolog.Logger, viewState *ViewState, voteValidator *Validator) *VoteAggregator {
	return &VoteAggregator{
		log:                     log,
		viewState:               viewState,
		voteValidator:           voteValidator,
		blockHashToVotingStatus: map[string]*VotingStatus{},
	}
}

// StorePendingVote stores the vote as a pending vote assuming the caller has checked that the voting
// block is currently missing.
// Note: Validations on these pending votes will be postponed until the block has been received.
func (va *VoteAggregator) StorePendingVote(vote *types.Vote) error {
	if vote.View <= va.lastPrunedView {
		return fmt.Errorf("the vote is stale: %w", types.ErrStaleVote{
			Vote:          vote,
			FinalizedView: va.lastPrunedView,
		})
	}
	voteMap, exists := va.pendingVotes[vote.BlockMRH.String()]
	if exists {
		voteMap[vote.Hash()] = vote
	} else {
		va.pendingVotes[vote.BlockMRH.String()] = make(map[string]*types.Vote)
		va.pendingVotes[vote.BlockMRH.String()][vote.Hash()] = vote
	}
	return nil
}

// StoreVoteAndBuildQC stores the vote assuming the caller has checked that the voting block is incorporated,
// and returns a QC if there are votes with enough stakes.
// The VoteAggregator builds a QC as soon as the number of votes allow this.
// While subsequent votes (past the required threshold) are not included in the QC anymore,
// VoteAggregator ALWAYS returns the same QC as the one returned before.
func (va *VoteAggregator) StoreVoteAndBuildQC(vote *types.Vote, bp *types.BlockProposal) (*types.QuorumCertificate, error) {
	// if the QC for the block has been created before, return the QC
	oldQC, built := va.createdQC[fmt.Sprintf("%x", bp.BlockMRH())]
	if built {
		return oldQC, nil
	}
	// ignore stale votes
	if vote.View <= va.lastPrunedView {
		return nil, fmt.Errorf("the vote is stale: %w", types.ErrStaleVote{
			Vote:          vote,
			FinalizedView: va.lastPrunedView,
		})
	}
	va.log.Info().Msg("new incorporated vote added")
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

// BuildQCForBlockProposal will extract a primary vote out of the block proposal and
// attempt to build a QC for the given block proposal when there are votes
// with enough stakes.
func (va *VoteAggregator) BuildQCOnReceivingBlock(bp *types.BlockProposal) (*types.QuorumCertificate, *multierror.Error) {
	var result *multierror.Error
	oldQC, built := va.createdQC[fmt.Sprintf("%x", bp.BlockMRH())]
	if built {
		return oldQC, nil
	}
	for _, vote := range va.pendingVotes[bp.BlockMRH().String()] {
		voteStatus, err := va.validateAndStoreIncorporatedVote(vote, bp)
		if err != nil {
			va.log.Debug().Msg("invalid vote found")
			result = multierror.Append(result, fmt.Errorf("could not save pending vote: %w", err))
		}
		voteStatus.AddVote(vote)
	}

	primaryVote := types.NewVote(bp.View(), bp.BlockMRH(), bp.Signature)
	qc, err := va.StoreVoteAndBuildQC(primaryVote, bp)
	if err != nil {
		result = multierror.Append(result, fmt.Errorf("could not build QC on receiving block proposal: %w", err))
		return nil, result
	}

	return qc, result
}

// PruneByView will delete all votes equal or below to the given view, as well as related indexes.
func (va *VoteAggregator) PruneByView(view uint64) {
	for i := va.lastPrunedView + 1; i <= view; i++ {
		blockMRHs := va.viewToBlockMRH[i]
		for _, blockMRH := range blockMRHs {
			blockMRHStr := string(blockMRH)
			delete(va.viewToBlockMRH, i)
			delete(va.pendingVotes, blockMRHStr)
			delete(va.blockHashToVotingStatus, blockMRHStr)
			delete(va.createdQC, blockMRHStr)
			delete(va.viewToIDToVote, view)
		}
	}
	va.lastPrunedView = view

	va.log.Info().Msg("successfully pruned")
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
		return nil, fmt.Errorf("double voting detected: %w", err)
	}
	// update existing voting status or create a new one
	votingStatus, exists := va.blockHashToVotingStatus[vote.BlockMRH.String()]
	if !exists {
		threshold, err := va.viewState.GetQCStakeThresholdForBlockID(votingStatus.BlockID())
		if err != nil {
			return nil, fmt.Errorf("could not get stake threshold %w", err)
		}
		votingStatus = NewVotingStatus(threshold, voter, vote.BlockMRH)
		va.blockHashToVotingStatus[vote.BlockMRH.String()] = votingStatus
	}
	votingStatus.AddVote(vote)
	va.viewToIDToVote[vote.View][voter.ID()] = vote
	return votingStatus, nil
}

func (va *VoteAggregator) tryBuildQC(votingStatus *VotingStatus) (*types.QuorumCertificate, error) {
	if !votingStatus.CanBuildQC() {
		return nil, types.ErrInsufficientVotes{}
	}
	// TODO: to create aggregate sig
	qc := &types.QuorumCertificate{}
	va.createdQC[votingStatus.blockMRH.String()] = qc
	va.log.Info().Msg("new QC created")
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
	if originalVote.BlockMRH == vote.BlockMRH {
		// voted and is the same vote as the vote received before
		return nil
	}
	return types.ErrDoubleVote{
		OriginalVote: originalVote,
		DoubleVote:   vote,
	}
}

func getSigsSliceFromVotes(votes map[string]*types.Vote) []*types.Signature {
	var signatures = make([]*types.Signature, len(votes))
	i := 0
	for _, vote := range votes {
		signatures[i] = vote.Signature
		i++
	}

	return signatures
}
