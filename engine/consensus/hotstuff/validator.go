package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type Validator struct {
	viewState ViewState
}

func (v *Validator) ValidateQC(qc *types.QuorumCertificate) bool {
	panic("TODO")
}

func (v *Validator) ValidateBlock(bp *types.BlockProposal) bool {
	panic("TODO")
}

func (v *Validator) ValidateVote(vote *types.Vote) bool {
	panic("TODO")
}
