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
	viewState      ViewState
	voteValidator  Validator
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
	viewToIDToVote map[uint64]map[string]*types.Vote
}

func NewVoteAggregator(log zerolog.Logger, viewState ViewState, voteValidator Validator) *VoteAggregator {
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
	_, err := va.voteValidator.ValidateVote(vote, nil)
	if err != nil {
		return fmt.Errorf("could not validate the vote: %w", err)
	}

	if vote.View <= va.lastPrunedView {
		return fmt.Errorf("the vote is stale: %w", types.ErrStaleVote{
			Vote:          vote,
			FinalizedView: va.lastPrunedView,
		})
	}

	va.pendingVotes[string(vote.BlockMRH)][vote.Hash()] = vote
	va.log.Info().Msg("new pending vote added")
	return nil
}

// StoreVoteAndBuildQC stores the vote assuming the caller has checked that the voting block is incorporated,
// and returns a QC if there are votes with enough stakes.
// The VoteAggregator builds a QC as soon as the number of votes allow this.
// While subsequent votes (past the required threshold) are not included in the QC anymore,
// VoteAggregator ALWAYS returns the same QC as the one returned before.
func (va *VoteAggregator) StoreVoteAndBuildQC(vote *types.Vote, bp *types.BlockProposal) (*types.QuorumCertificate, error) {
	err := va.storeIncorporatedVote(vote, bp)
	if err != nil {
		return nil, fmt.Errorf("could not store incorporated vote: %w", err)
	}

	// if the QC for the block has been created before, return the QC
	oldQC, built := va.createdQC[string(bp.Block.BlockMRH())]
	if built {
		return oldQC, nil
	}

	newQC, err := va.buildQC(bp.Block)
	if err != nil {
		return nil, fmt.Errorf("could not build QC: %w", err)
	}

	va.log.Info().Msg("new QC created")

	return newQC, nil
}

// BuildQCForBlockProposal will extract a primary vote out of the block proposal and
// attempt to build a QC for the given block proposal when there are votes
// with enough stakes.
func (va *VoteAggregator) BuildQCOnReceivingBlock(bp *types.BlockProposal) (*types.QuorumCertificate, *multierror.Error) {
	var result *multierror.Error
	for _, vote := range va.pendingVotes[string(bp.Block.BlockMRH())] {
		err := va.storeIncorporatedVote(vote, bp)
		if err != nil {
			va.log.Debug().Msg("invalid pending vote found")
			result = multierror.Append(result, fmt.Errorf("could not save pending vote: %w", err))
		}
	}

	primaryVote := types.NewVote(bp.View(), bp.BlockMRH(), bp.Signature)
	qc, err := va.StoreVoteAndBuildQC(primaryVote, bp)
	if err != nil {
		result = multierror.Append(result, fmt.Errorf("could not build QC on receiving block proposal: %w", err))
		return nil, result
	}

	va.log.Info().Msg("new QC created")

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
func (va *VoteAggregator) storeIncorporatedVote(vote *types.Vote, bp *types.BlockProposal) error {
	voter, err := va.voteValidator.ValidateVote(vote, bp)
	if err != nil {
		return fmt.Errorf("could not validate incorporated vote: %w", err)
	}

	if vote.View <= va.lastPrunedView {
		return fmt.Errorf("the vote is stale: %w", types.ErrStaleVote{
			Vote:          vote,
			FinalizedView: va.lastPrunedView,
		})
	}

	originalVote, ok := va.isDoubleVote(vote, voter)
	if ok {
		return fmt.Errorf("double voting detected: %w", types.ErrDoubleVote{
			OriginalVote: originalVote,
			DoubleVote:   vote,
		})
	}
	// update existing voting status or create a new one
	votingStatus, exists := va.blockHashToVotingStatus[string(vote.BlockMRH)]
	if !exists {
		threshold := va.viewState.ComputeQCStakeThresholdAtBlockID(vote.BlockMRH)
		votingStatus = NewVotingStatus(threshold, voter)
		va.blockHashToVotingStatus[string(vote.BlockMRH)] = votingStatus
	}
	votingStatus.AddVote(vote)
	va.viewToIDToVote[vote.View][voter.String()] = vote
	va.log.Info().Msg("new incorporated vote added")
	return nil
}

func (va *VoteAggregator) buildQC(block *types.Block) (*types.QuorumCertificate, error) {
	blockMRHStr := string(block.BlockMRH())
	votingStatus := va.blockHashToVotingStatus[blockMRHStr]
	if !votingStatus.canBuildQC() {
		va.log.Debug().Msg("vote threshold not reached")
		return nil, fmt.Errorf("can not build a QC: %w", types.ErrInsufficientVotes{})
	}

	sigs := getSigsSliceFromVotes(va.blockHashToVotingStatus[blockMRHStr].validVotes)
	qc := types.NewQC(block, sigs)
	va.createdQC[blockMRHStr] = qc

	return qc, nil
}

// double voting is detected when the voter has voted a different block at the same view before
func (va *VoteAggregator) isDoubleVote(vote *types.Vote, sender *flow.Identity) (*types.Vote, bool) {
	if _, ok := va.viewToIDToVote[vote.View]; ok {
		originalVote, exists := va.viewToIDToVote[vote.View][sender.String()]
		if exists {
			if string(originalVote.BlockMRH) != string(vote.BlockMRH) {
				return originalVote, true
			}
		}
	}
	return nil, false
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
