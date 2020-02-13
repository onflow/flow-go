package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Validator struct {
	viewState *ViewState
}

// ValidateQC validates the QC
// It doesn't validate the block that this QC is pointing to
func (v *Validator) ValidateQC(qc *types.QuorumCertificate) error {
	_ = v.viewState // suppress unused warning
	panic("TODO")
}

// ValidateProposal validates the block header, returns the signer and a validated block proposal
// bp - the block header to be validated.
// parent - the parent of the block proposal.
func (v *Validator) ValidateProposal(proposal *types.Proposal) error {
	panic("TODO")
}

// ValidateVote validates the vote and returns the signer identity who signed the vote
// vote - the vote to be validated
// bp - the voting block
func (v *Validator) ValidateVote(vote *types.Vote, block *types.Block) (*flow.Identity, error) {
	panic("TODO")
}
