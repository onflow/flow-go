package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type Voter struct {
	signer     Signer
	viewState  ViewState
	forkChoice *ForkChoice
	// Flag to turn on/off consensus acts (voting, block production etc)
	isConActor bool
}

func (v *Voter) VoteIfVotable(bp *types.BlockProposal, curView uint64) (*types.Vote, bool) {
	if v.forkChoice.IsSafeNode(bp) {
		// TODO: log this reason
		return nil, false
	}

	if curView != bp.Block.View {
		// TODO: log this reason
		return nil, false
	}

	if !v.isConActor {
		// TODO: log this reason
		return nil, false
	}

	return v.produceVote(bp), true
}

func (v *Voter) produceVote(bp *types.BlockProposal) *types.Vote {
	signerIdx := v.viewState.GetSelfIdxForView(bp.Block.View)
	unsignedVote := types.NewVote(bp.Block.View, bp.Block.BlockMRH())
	vote := v.signer.SignVote(unsignedVote, signerIdx)

	return vote
}
