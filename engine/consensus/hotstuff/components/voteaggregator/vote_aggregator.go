package voteaggregator

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	hotstuff "github.com/dapperlabs/flow-go/engine/consensus/HotStuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
)

type VoteAggregator struct {
	log           zerolog.Logger
	protocolState protocol.State
	viewState     hotstuff.ViewState
	voteValidator hotstuff.Validator
	// For pruning
	viewToBlockMRH map[uint64][]byte
	// keeps track of votes whose blocks can not be found
	pendingVotes map[string]map[string]*types.Vote
	// keeps track of QCs that have been made for blocks
	createdQC map[string]*types.QuorumCertificate
	// keeps track of accumulated votes and stakes for blocks
	blockHashToVotingStatus map[string]*VotingStatus
}

func NewVoteAggregator(log zerolog.Logger, protocolState protocol.State, viewState hotstuff.ViewState, voteValidator hotstuff.Validator) *VoteAggregator {
	return &VoteAggregator{
		log:           log,
		protocolState: protocolState,
		viewState:     viewState,
		voteValidator: voteValidator,
	}
}

// StorePendingVote stores the vote as a pending vote assuming the caller has checked that the voting
// block is currently missing.
// Note: Validations on these pending votes will be postponed until the block has been received.
func (va *VoteAggregator) StorePendingVote(vote *types.Vote) error {
	err := va.voteValidator.ValidatePendingVote(vote)
	if err != nil {
		return fmt.Errorf("could not validate the vote: %w", err)
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

// BuildQCForBlockProposal will attempt to build a QC for the given block proposal when there are votes
// with enough stakes.
// VoteAggregator ALWAYS returns the same QC as the one returned before.
func (va *VoteAggregator) BuildQCOnReceivingBlock(bp *types.BlockProposal) (*types.QuorumCertificate, *multierror.Error) {
	var result *multierror.Error
	for _, vote := range va.pendingVotes[string(bp.Block.BlockMRH())] {
		err := va.storeIncorporatedVote(vote, bp)
		if err != nil {
			va.log.Debug().Msg("invalid pending vote found")
			result = multierror.Append(result, fmt.Errorf("could not save pending vote: %w", err))
		}
	}

	qc, err := va.buildQC(bp.Block)
	if err != nil {
		result = multierror.Append(result, fmt.Errorf("could not build QC: %w", err))
		return nil, result
	}

	va.log.Info().Msg("new QC created")

	return qc, result
}

// PruneByView will delete all votes equal or below to the given view, as well as related indexes.
func (va *VoteAggregator) PruneByView(view uint64) {
	if view < 1 {
		return
	}
	blockMRHStr := string(va.viewToBlockMRH[view-1])
	delete(va.viewToBlockMRH, view)
	delete(va.pendingVotes, blockMRHStr)
	delete(va.blockHashToVotingStatus, blockMRHStr)
	delete(va.createdQC, blockMRHStr)
	va.log.Info().Msg("successfully pruned")
}

// storeIncorporatedVote stores incorporated votes and accumulate stakes
// it drops invalid votes and duplicate votes
func (va *VoteAggregator) storeIncorporatedVote(vote *types.Vote, bp *types.BlockProposal) error {
	// TODO: will be using AtBlockID after unifying Block structure
	identities, err := va.protocolState.AtNumber(bp.Block.View).Identities(identity.HasRole(flow.RoleConsensus))
	if err != nil {
		return fmt.Errorf("cannot get protocol state at block")
	}

	err = va.voteValidator.ValidateIncorporatedVote(vote, bp, identities)
	if err != nil {
		return fmt.Errorf("could not validate incorporated vote: %w", err)
	}

	voteSender := identities.Get(uint(vote.Signature.SignerIdx))
	// update existing voting status or create a new one
	votingStatus, exists := va.blockHashToVotingStatus[string(vote.BlockMRH)]
	if exists {
		votingStatus.validVotes[vote.Hash()] = vote
		votingStatus.accumulatedStake += voteSender.Stake
	} else {
		votingStatus = &VotingStatus{
			thresholdStake:   uint64(float32(identities.TotalStake())*va.viewState.ThresholdStake(vote.View)) + 1,
			accumulatedStake: voteSender.Stake,
			validVotes:       map[string]*types.Vote{vote.Hash(): vote},
		}
		va.blockHashToVotingStatus[string(vote.BlockMRH)] = votingStatus
	}

	va.log.Info().Msg("new incorporated vote added")
	return nil
}

func (va *VoteAggregator) buildQC(block *types.Block) (*types.QuorumCertificate, error) {
	identities, err := va.protocolState.AtNumber(block.View).Identities(identity.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, fmt.Errorf("could not get protocol state %w", err)
	}

	blockMRHStr := string(block.BlockMRH())
	votingStatus := va.blockHashToVotingStatus[blockMRHStr]
	if !votingStatus.canBuildQC() {
		va.log.Debug().Msg("vote threshold not reached")
		return nil, fmt.Errorf("can not build a QC: %w", errInsufficientVotes{})
	}

	sigs := getSigsSliceFromVotes(va.pendingVotes[blockMRHStr])
	qc := types.NewQC(block, sigs, uint32(identities.Count()))
	va.createdQC[blockMRHStr] = qc

	return qc, nil
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
