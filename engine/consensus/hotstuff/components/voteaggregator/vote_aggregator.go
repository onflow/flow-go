package voteaggregator

import (
	"fmt"
	"sync"

	hotstuff "github.com/dapperlabs/flow-go/engine/consensus/HotStuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
)

type VotingStatus struct {
	thresholdStake   uint64
	accumulatedStake uint64
	validVotes       map[string]*types.Vote
}

type VoteAggregator struct {
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
	blockHashToVotingStatus map[string]VotingStatus
}

// StorePendingVote stores the vote as a pending vote assuming the caller has checked that the voting
// block is currently missing.
// Note: Validations on these pending votes will be postponed until the block has been received.
func (va *VoteAggregator) StorePendingVote(vote *types.Vote) error {
	va.pendingVotes[string(vote.BlockMRH)][vote.Hash()] = vote
}

// StoreVoteAndBuildQC stores the vote assuming the caller has checked that the voting block is incorporated,
// and returns a QC if there are votes with enough stakes.
// The VoteAggregator builds a QC as soon as the number of votes allow this.
// While subsequent votes (past the required threshold) are not included in the QC anymore,
// VoteAggregator ALWAYS returns the same QC as the one returned before.
func (va *VoteAggregator) StoreVoteAndBuildQC(vote *types.Vote, bp *types.BlockProposal) (*types.QuorumCertificate, error) {
	err := va.storeIncorporatedVote(vote, bp)
	if err != nil {
		return nil, fmt.Errorf("could not store incorporated vote %v", err)
	}

	if _, hasBuiltQC := va.createdQC[string(bp.Block.BlockMRH())]; hasBuiltQC == false {
		return va.buildQC(bp.Block)
	}

	// QC for the same block has been built before
	return nil, nil
}

// BuildQCForBlockProposal will attempt to build a QC for the given block proposal when there are votes
// with enough stakes.
// VoteAggregator ALWAYS returns the same QC as the one returned before.
func (va *VoteAggregator) BuildQCOnReceivingBlock(bp *types.BlockProposal) (*types.QuorumCertificate, bool) {
	var errors []error
	for _, vote := range va.pendingVotes[string(bp.Block.BlockMRH())] {
		if err := va.storeIncorporatedVote(vote, bp); err != nil {
			errors = append(errors, err)
		}
	}
	qc, err := va.buildQC(bp.Block)
	if err != nil {
		errors = append(errors, err)
	}
	return qc, false
}

// PruneByView will delete all votes equal or below to the given view, as well as related indexes.
func (va *VoteAggregator) PruneByView(view uint64) {
	if view > 1 {
		blockMRHStr := string(va.viewToBlockMRH[view-1])
		delete(va.viewToBlockMRH, view)
		delete(va.pendingVotes, blockMRHStr)
		delete(va.incorporatedVotes, blockMRHStr)
		delete(va.blockHashToIncorporatedStakes, blockMRHStr)
		delete(va.createdQC, blockMRHStr)
	}
}

func (va *VoteAggregator) shouldDropVote(vote *types.Vote) bool {
	// the vote is duplicate
	if _, existed := va.incorporatedVotes[string(vote.BlockMRH)][vote.Hash()]; existed {
		return true
	}
	// the QC for the same block has been created
	if _, isCreated := va.createdQC[string(vote.BlockMRH)]; isCreated {
		return true
	}

	return false
}

// storeIncorporatedVote stores incorporated votes and accumulate stakes
// it drops invalid votes and duplicate votes
func (va *VoteAggregator) storeIncorporatedVote(vote *types.Vote, bp *types.BlockProposal) error {
	identities, err := va.protocolState.AtNumber(bp.Block.View).Identities(identity.HasRole(flow.RoleConsensus))
	if err != nil {
		return fmt.Errorf("cannot get protocol state at block")
	}

	err := va.voteValidator.ValidateIncorporatedVote(vote, bp, identities)
	if err != nil {
		return fmt.Errorf("could not validate incorporated vote %v", err)
	}

	va.viewToBlockMRH[vote.View] = vote.BlockMRH
	va.incorporatedVotes[string(vote.BlockMRH)][vote.Hash()] = vote
	voteSender := identities.Get(uint(vote.Signature.SignerIdx))
	va.blockHashToIncorporatedStakes[string(vote.BlockMRH)] += voteSender.Stake
}

func (va *VoteAggregator) buildQC(block *types.Block) (*types.QuorumCertificate, error) {
	identities, err := va.protocolState.AtNumber(block.View).Identities(identity.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, fmt.Errorf("could not get protocol state %v", err)
	}

	blockMRHStr := string(block.BlockMRH())
	voteThreshold := uint64(float32(identities.TotalStake())*va.viewState.ThresholdStake()) + 1
	// upon receipt of sufficient votes (in terms of stake)
	if va.blockHashToIncorporatedStakes[blockMRHStr] >= voteThreshold {
		sigs := getSigsSliceFromVotes(va.pendingVotes[blockMRHStr])
		qc := types.NewQC(block, sigs, uint32(identities.Count()))
		va.createdQC[blockMRHStr] = qc
		return qc, nil
	}

	return nil, nil
}

func getSigsSliceFromVotes(votes []*types.Vote) []*types.Signature {
	var signatures = make([]*types.Signature, len(votes))
	for i, vote := range votes {
		signatures[i] = vote.Signature
	}

	return signatures
}
