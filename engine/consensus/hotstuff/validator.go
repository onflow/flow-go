package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/signature"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Validator struct {
	// Validator depends on ViewState to query the list of staked nodes on certain fork
	viewState ViewState
	// Validator depends on SigVerifier to verify signatures and aggregated signatures
	verifier signature.SigVerifier
}

// ValidateQC validates the QC
// It doesn't validate the block that this QC is pointing to
func (v *Validator) ValidateQC(qc *types.QuorumCertificate) error {
	panic("TODO")
}

// ValidateBlock validates the block header, returns the signer and a validated block proposal
// bp - the block header to be validated.
// parent - the parent of the block proposal.
func (v *Validator) ValidateBlock(bp *types.BlockHeader, parent *types.BlockProposal) (*flow.Identity, *types.BlockProposal, error) {
	panic("TODO")
}

// ValidateVote validates the vote and returns the signer identity who signed the vote
// vote - the vote to be validated
// bp - the voting block
func (v *Validator) ValidateVote(vote *types.Vote, bp *types.BlockProposal) (*flow.Identity, error) {
	panic("TODO")
}
