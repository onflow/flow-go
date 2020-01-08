package hotstuff

import (
	"sync"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type VoteAggregator struct {
	sync.RWMutex
	pendingVotes map[types.MRH][]*types.Vote
	createdQC    map[types.MRH]*types.QuorumCertificate
	viewState    ViewState
}

func (va *VoteAggregator) Store(v *types.Vote) {
	panic("TODO")
}

// only returns a QC if there is enough votes and it never makes such QC before
func (va *VoteAggregator) StoreAndMakeQCForIncorporatedVote(v *types.Vote, b *types.BlockProposal) (*types.QuorumCertificate, bool) {
	panic("TODO")
}

func (va *VoteAggregator) BuildQCForBlockProposal(b *types.BlockProposal) (*types.QuorumCertificate, bool) {
	panic("TODO")
}

func (va *VoteAggregator) PruneByView(view uint64) {
	panic("TODO")
}
