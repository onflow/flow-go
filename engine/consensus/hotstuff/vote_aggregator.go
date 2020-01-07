package hotstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/protocol"
	"sync"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type VoteAggregator struct {
	lock  sync.RWMutex
	state protocol.State
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

// store pending votes
func (va *VoteAggregator) StorePendingVote(vote *types.Vote) {
	va.pendingVotes[vote.BlockMRH] = append(va.pendingVotes[vote.BlockMRH], vote)
}

// store incorporated votes and accumulate stakes
func (va *VoteAggregator) StoreIncorporatedVote(vote *types.Vote) {
	va.viewToBlockMRH[vote.View] = vote.BlockMRH
	va.incorporatedVotes[vote.BlockMRH] = append(va.incorporatedVotes[vote.BlockMRH], vote)
	va.accumulateStakes(vote)
}

// MakeQCForBlock only returns a QC if there are enough votes or the QC for the block has been made before
func (va *VoteAggregator) MakeQCForBlock(block *types.Block) *types.QuorumCertificate {
	if _, hasBuiltQC := va.createdQC[block.BlockMRH]; hasBuiltQC {
		return nil
	}
	if identities, err := va.state.AtHash(block.BlockMRH[:]).Identities(identity.HasRole(flow.RoleConsensus)); err != nil {
		qc := va.checkThresholdAndGenerateQC(block, identities)
		return qc
	} else {
		panic("can't get stake for the block")
		return nil
	}
}

// BuildQCOnReceivingBlock handles pending votes if there are any and then try to make a QC
// returns nil if there are no pending votes or the accumulated stakes are not enough to build a QC
func (va *VoteAggregator) BuildQCOnReceivingBlock(block *types.Block) *types.QuorumCertificate {
	if _, hasPendingVotes := va.pendingVotes[block.BlockMRH]; hasPendingVotes {
		va.handlePendingVotesForBlock(block.BlockMRH)
		return va.MakeQCForBlock(block)
	}

	// there are no pending votes, no need to create a QC
	return nil
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
		va.StoreIncorporatedVote(vote)
	}
}

func (va *VoteAggregator) accumulateStakes(vote *types.Vote) {
	if identities, err := va.state.AtHash(vote.BlockMRH[:]).Identities(identity.HasRole(flow.RoleConsensus)); err != nil {
		voteSender := identities.Get(uint(vote.Signature.SignerIdx))
		va.blockHashToIncorporatedStakes[vote.BlockMRH] += voteSender.Stake
	}
}

// build a QC when reaching the vote threshold
func (va *VoteAggregator) checkThresholdAndGenerateQC(block *types.Block, identities flow.IdentityList) *types.QuorumCertificate {
	voteThreshold := identities.TotalStake()*2/3 + 1
	// upon receipt of sufficient votes (in terms of stake)
	if va.blockHashToIncorporatedStakes[block.BlockMRH] >= voteThreshold {
		sigs := getSigsSliceFromVotes(va.pendingVotes[block.BlockMRH])
		qc := va.buildQC(block, sigs, identities)

		return qc
	}

	return nil
}

func getSigsSliceFromVotes(votes []*types.Vote) []*types.Signature {
	var signatures = make([]*types.Signature, len(votes))
	for i, vote := range votes {
		signatures[i] = &vote.Signature
	}

	return signatures
}

func (va *VoteAggregator) buildQC(block *types.Block, sigs []*types.Signature, identities flow.IdentityList) *types.QuorumCertificate {
	qc := types.NewQC(block, sigs, uint32(identities.Count()))
	va.createdQC[block.BlockMRH] = qc

	return qc
}
