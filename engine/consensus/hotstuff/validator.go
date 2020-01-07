package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type Validator struct {
	protocolState ProtocolState
}

func (v *Validator) ValidateQC(qc *types.QuorumCertificate) bool {
	panic("TODO")
}

func (v *Validator) ValidateBlock(qc *types.QuorumCertificate, bp *types.BlockProposal) bool {
	panic("TODO")
}

func (v *Validator) ValidateIncorporatedVote(vote *types.Vote, bp *types.BlockProposal) bool {
	panic("TODO")
}

func (v *Validator) ValidatePendingVote(vote *types.Vote) bool {
	panic("TODO")
}
