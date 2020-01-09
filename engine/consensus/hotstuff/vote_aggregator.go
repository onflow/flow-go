package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
	"sync"
)

type VoteAggregator struct {
	lock          sync.RWMutex
	protocolState protocol.State
	viewState     ViewState
	// For pruning
	viewToBlockMRH map[uint64]types.MRH
	// keeps track of votes whose blocks can not be found
	pendingVotes map[types.MRH][]*types.Vote
	// keeps track of votes that are valid to accumulate stakes
	incorporatedVotes map[types.MRH][]*types.Vote
	// keeps track of QCs that have been made for blocks
	createdQC map[types.MRH]*types.QuorumCertificate
	// keeps track of accumulated stakes for blocks
	blockHashToIncorporatedStakes map[types.MRH]uint64
}

// Store stores the vote in internal memory separately depending on if it is pending
func (va *VoteAggregator) Store(vote *types.Vote, isPending bool) {
	if isPending {
		va.storePendingVote(vote)
	} else {
		va.storeIncorporatedVote(vote)
	}
}

// store pending votes
func (va *VoteAggregator) storePendingVote(vote *types.Vote) {
	va.pendingVotes[vote.BlockMRH] = append(va.pendingVotes[vote.BlockMRH], vote)
}

// StoreIncorporatedVote stores incorporated votes and accumulate stakes
// if the QC for the same view has been created, ignore subsequent votes
func (va *VoteAggregator) storeIncorporatedVote(vote *types.Vote) {
	if blockMRH, exists := va.viewToBlockMRH[vote.View]; exists {
		if _, isCreated := va.createdQC[blockMRH]; isCreated {
			return
		}
	}
	va.viewToBlockMRH[vote.View] = vote.BlockMRH
	va.incorporatedVotes[vote.BlockMRH] = append(va.incorporatedVotes[vote.BlockMRH], vote)
	va.accumulateStakes(vote)
}

// StoreVoteAndBuildQC adds the vote to the VoteAggregator internal memory and returns a QC if there are enough votes.
// The VoteAggregator builds a QC as soon as the number of votes allow this.
// While subsequent votes (past the required threshold) are not included in the QC anymore,
// VoteAggregator ALWAYS returns a QC is possible.
func (va *VoteAggregator) StoreVoteAndBuildQC(vote *types.Vote, block *types.Block) (*types.QuorumCertificate, bool) {
	va.storeIncorporatedVote(vote)
	if _, hasBuiltQC := va.createdQC[block.BlockMRH]; hasBuiltQC == false {
		return va.buildQC(block)
	}

	return nil, false
}

// BuildQCOnReceivingBlock handles pending votes if there are any and then try to make a QC
// returns nil if there are no pending votes or the accumulated stakes are not enough to build a QC
func (va *VoteAggregator) BuildQCOnReceivingBlock(block *types.Block) (*types.QuorumCertificate, bool) {
	if _, hasPendingVotes := va.pendingVotes[block.BlockMRH]; hasPendingVotes == true {
		va.handlePendingVotesForBlock(block.BlockMRH)
		return va.buildQC(block)
	}

	// there are no pending votes, no need to create a QC
	return nil, false
}

// garbage collection by view
func (va *VoteAggregator) PruneByView(view uint64) {
	blockMRH := va.viewToBlockMRH[view]
	delete(va.viewToBlockMRH, view)
	delete(va.pendingVotes, blockMRH)
	delete(va.incorporatedVotes, blockMRH)
	delete(va.blockHashToIncorporatedStakes, blockMRH)
	delete(va.createdQC, blockMRH)
}

// move pending votes to incorporated votes and accumulate stakes
func (va *VoteAggregator) handlePendingVotesForBlock(blockMRH types.MRH) {
	for _, vote := range va.pendingVotes[blockMRH] {
		va.storeIncorporatedVote(vote)
	}
}

func (va *VoteAggregator) accumulateStakes(vote *types.Vote) {
	identities, err := va.protocolState.AtHash(vote.BlockMRH[:]).Identities(identity.HasRole(flow.RoleConsensus))
	if err == nil {
		voteSender := identities.Get(uint(vote.Signature.SignerIdx))
		va.blockHashToIncorporatedStakes[vote.BlockMRH] += voteSender.Stake
	}
}

func (va *VoteAggregator) buildQC(block *types.Block) (*types.QuorumCertificate, bool) {
	identities, err := va.protocolState.AtHash(block.BlockMRH[:]).Identities(identity.HasRole(flow.RoleConsensus))
	if err != nil {
		panic("can't get stake for the block")
		return nil, false
	}
	voteThreshold := uint64(float32(identities.TotalStake())*va.viewState.ThresholdStake()) + 1
	// upon receipt of sufficient votes (in terms of stake)
	if va.blockHashToIncorporatedStakes[block.BlockMRH] >= voteThreshold {
		sigs := getSigsSliceFromVotes(va.pendingVotes[block.BlockMRH])
		qc := types.NewQC(block, sigs, uint32(identities.Count()))
		va.createdQC[block.BlockMRH] = qc
		return qc, true
	}

	return nil, false
}

func getSigsSliceFromVotes(votes []*types.Vote) []*types.Signature {
	var signatures = make([]*types.Signature, len(votes))
	for i, vote := range votes {
		signatures[i] = vote.Signature
	}

	return signatures
}
