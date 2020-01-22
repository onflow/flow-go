package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type Validator struct {
	viewState ViewState
}

// QC is valid (signature is valid and has enough stake)
func (v *Validator) ValidateQC(qc *types.QuorumCertificate) error {
	panic("TODO")
}

// ValidateBlock validates the block proposal
func (v *Validator) ValidateBlock(bp *types.BlockProposal) error {
	// proposer signature (valid and from the expected proposer)

	qc := bp.QC()

	err := v.ValidateQC(qc)
	if err != nil {
		return err
	}

	// Tuple (QC.view, QC.BlockID) references known block
	// height = parent height + 1
	// block.view > qc.view
}

func (v *Validator) ValidateVote(vote *types.Vote) error {
	panic("TODO")
}
