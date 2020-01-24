package hotstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type Validator struct {
	viewState ViewState
}

func (v *Validator) ValidateQC(qc *types.QuorumCertificate) bool {
	panic("TODO")
}

func (v *Validator) ValidateBlock(bp *types.BlockProposal) bool {
	panic("TODO")
}

func (v *Validator) ValidateVote(vote *types.Vote, bp *types.BlockProposal) (*flow.Identity, error) {
	//err := v.checkVoteSig(vote)
	//if err != nil {
	//	return nil, fmt.Errorf("could not validate the signature: %w", err)
	//}
	//
	//if bp == nil {
	//	return nil, nil
	//}
	//
	//if vote.View != bp.View() {
	//	return nil, fmt.Errorf("could not validate the view: %w", types.ErrInvalidView{vote})
	//}

	var id flow.Identifier
	v.viewState.Get
	copy(id[:], vote.Signature.i)

	identity := &flow.Identity{
		NodeID:  id,
		Address: "address",
		Role:    flow.RoleConsensus,
		Stake:   1000,
	}
	return identity, nil
}

func (v *Validator) checkVoteSig(vote *types.Vote) error {
	return nil
}
