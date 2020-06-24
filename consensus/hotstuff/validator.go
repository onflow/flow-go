package hotstuff

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Validator provides functions to validate QC, proposals and votes.
type Validator interface {

	// ValidateQC checks the validity of a QC for a given block.
	ValidateQC(qc *model.QuorumCertificate, block *model.Block) error

	// ValidateProposal checks the validity of a proposal.
	ValidateProposal(proposal *model.Proposal) error

	// ValidateVote checks the validity of a vote for a given block.
	ValidateVote(vote *model.Vote, block *model.Block) (*flow.Identity, error)
}
