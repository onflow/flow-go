package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type Voter struct {
	signer    Signer
	viewState ViewState
	forks     Forks
	// Flag to turn on/off consensus acts (voting, block production etc)
	isConActor bool
	// Need to keep track of the last view we voted for so we don't double vote accidentally
	lastVotedView uint64
}

func (v *Voter) ProduceVoteIfVotable(bp *types.BlockProposal, curView uint64) (*types.Vote, bool) {
	if !v.isConActor {
		// TODO: log this reason
		return nil, false
	}

	if v.forks.IsSafeNode(bp) {
		// TODO: log this reason
		return nil, false
	}

	if curView != bp.Block.View {
		// TODO: log this reason
		return nil, false
	}

	if curView <= v.lastVotedView {
		// TODO: log this reason
		return nil, false
	}

	return v.produceVote(bp), true
}

func (v *Voter) produceVote(bp *types.BlockProposal) *types.Vote {
	signerIdx := v.viewState.GetSelfIdxForView(bp.Block.View)
	unsignedVote := types.NewUnsignedVote(bp.Block.View, bp.Block.BlockMRH())
	sig := v.signer.SignVote(unsignedVote, signerIdx)

	return types.NewVote(bp.Block.View, bp.Block.BlockMRH(), sig)
}
