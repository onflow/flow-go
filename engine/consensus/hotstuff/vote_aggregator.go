package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type VoteAggregator struct {
	pendingVotes map[string][]*types.Vote
	createdQC    map[string]*types.QuorumCertificate
	viewState    ViewState
}

func (va *VoteAggregator) Store(v *types.Vote) {
	panic("TODO")
}

// StoreVoteAndBuildQC adds the vote to the VoteAggregator internal memory and returns a QC if there are enough votes.
// The VoteAggregator builds a QC as soon as the number of votes allow this.
// While subsequent votes (past the required threshold) are not included in the QC anymore,
// VoteAggregator ALWAYS returns a QC is possible.
func (va *VoteAggregator) StoreVoteAndBuildQC(v *types.Vote, b *types.BlockProposal) (*types.QuorumCertificate, bool) {
	panic("TODO")
}

func (va *VoteAggregator) BuildQCForBlockProposal(b *types.BlockProposal) (*types.QuorumCertificate, bool) {
	panic("TODO")
}

func (va *VoteAggregator) PruneByView(view uint64) {
	panic("TODO")
}
