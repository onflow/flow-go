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

type VoteAggregator struct {
	lock          sync.RWMutex
	protocolState protocol.State
	viewState     hotstuff.ViewState
	validator     hotstuff.Validator
	// For pruning
	viewToBlockMRH map[uint64][]byte
	// keeps track of votes whose blocks can not be found
	pendingVotes map[string][]*types.Vote
	// keeps track of votes that are valid to accumulate stakes
	incorporatedVotes map[string][]*types.Vote
	// keeps track of QCs that have been made for blocks
	createdQC map[string]*types.QuorumCertificate
	// keeps track of accumulated stakes for blocks
	blockHashToIncorporatedStakes map[string]uint64
}

func (va *VoteAggregator) StorePendingVote(vote *types.Vote) error {
	err := va.validator.ValidatePendingVote(vote)
	if err != nil {
		return fmt.Errorf("could not validate pending vote: %v", err)
	}

	va.pendingVotes[string(vote.BlockMRH)] = append(va.pendingVotes[string(vote.BlockMRH)], vote)

	return nil
}

// StoreIncorporatedVote stores incorporated votes and accumulate stakes
// if the QC for the same view has been created, ignore subsequent votes
func (va *VoteAggregator) storeIncorporatedVote(vote *types.Vote, bp *types.BlockProposal) error {
	// return error when the QC with the same view of the vote has been created
	if blockMRH, exists := va.viewToBlockMRH[vote.View]; exists {
		if qc, isCreated := va.createdQC[string(blockMRH)]; isCreated {
			return qcExistedError{
				vote,
				qc,
				fmt.Sprintf("QC for view %v has been created", vote.View)}
		}
	}

	// return error when cannot get protocol state
	identities, err := va.protocolState.AtNumber(bp.Block.View).Identities(identity.HasRole(flow.RoleConsensus))
	if err != nil {
		return fmt.Errorf("could not get protocol state %v", err)
	}

	// return error when the vote is invalid or cannot accumulate stakes
	err = va.validator.ValidateIncorporatedVote(vote, bp, identities)
	if err != nil {
		return fmt.Errorf("could not validate incorporated vote %v", err)
	}

	va.viewToBlockMRH[vote.View] = vote.BlockMRH
	va.incorporatedVotes[string(vote.BlockMRH)] = append(va.incorporatedVotes[string(vote.BlockMRH)], vote)
	err = va.accumulateStakes(vote, bp)
	if err != nil {
		return fmt.Errorf("could not accumulate stake %v", err)
	}

	return nil
}

// StoreVoteAndBuildQC adds the vote to the VoteAggregator internal memory and returns a QC if there are enough votes.
// The VoteAggregator builds a QC as soon as the number of votes allow this.
// While subsequent votes (past the required threshold) are not included in the QC anymore,
// VoteAggregator ALWAYS returns a QC is possible.
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

// BuildQCOnReceivingBlock handles pending votes if there are any and then try to make a QC
// returns nil if there are no pending votes or the accumulated stakes are not enough to build a QC
// returns a list
func (va *VoteAggregator) BuildQCOnReceivingBlock(bp *types.BlockProposal) (*types.QuorumCertificate, []error) {
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
	return qc, errors
}

// garbage collection by view
func (va *VoteAggregator) PruneByView(view uint64) {
	blockMRHStr := string(va.viewToBlockMRH[view])
	delete(va.viewToBlockMRH, view)
	delete(va.pendingVotes, blockMRHStr)
	delete(va.incorporatedVotes, blockMRHStr)
	delete(va.blockHashToIncorporatedStakes, blockMRHStr)
	delete(va.createdQC, blockMRHStr)
}

func (va *VoteAggregator) accumulateStakes(vote *types.Vote, bp *types.BlockProposal) error {
	identities, err := va.protocolState.AtNumber(bp.Block.View).Identities(identity.HasRole(flow.RoleConsensus))
	if err != nil {
		return err
	}

	voteSender := identities.Get(uint(vote.Signature.SignerIdx))
	va.blockHashToIncorporatedStakes[string(vote.BlockMRH)] += voteSender.Stake

	return nil
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
