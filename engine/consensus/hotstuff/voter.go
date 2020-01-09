package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type Voter struct {
	signer    Signer
	viewState ViewState
}

func (v *Voter) ProduceVote(bp *types.BlockProposal) (myVote *types.Vote) {
	signerIdx := v.viewState.GetSelfIdxForView(bp.Block.View)
	unsignedVote := types.NewVote(bp.Block.View, bp.Block.BlockMRH())
	vote := v.signer.SignVote(unsignedVote, signerIdx)

	return vote
}
