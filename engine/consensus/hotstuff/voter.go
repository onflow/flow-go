package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type Voter struct {
	signer Signer
}

func (v *Voter) ProduceVote(bp *types.BlockProposal) (myVote *types.Vote) {
	unsignedVote := types.NewVote(bp.Block.View, bp.Block.BlockMRH())
	vote := v.signer.SignVote(unsignedVote)

	return vote
}
