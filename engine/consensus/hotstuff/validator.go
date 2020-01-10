package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Validator struct {
	viewState ViewState
}

func (v *Validator) ValidateQC(qc *types.QuorumCertificate) bool {
	panic("TODO")
}

func (v *Validator) ValidateBlock(qc *types.QuorumCertificate, bp *types.BlockProposal) bool {
	panic("TODO")
}

func (v *Validator) ValidateIncorporatedVote(vote *types.Vote, bp *types.BlockProposal, identities flow.IdentityList) (bool, error) {
	panic("TODO")
}

func (v *Validator) ValidatePendingVote(vote *types.Vote) (bool, error) {
	panic("TODO")
}
