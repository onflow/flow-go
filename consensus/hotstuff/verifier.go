package hotstuff

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
)

// Verifier is the component responsible for validating votes, proposals and
// QC's against the block they are based on.
type Verifier interface {

	// VerifyProposal checks the validity of a given proposal.
	VerifyProposal(proposal *model.Proposal) (bool, error)

	// VerifyVote checks the validity of a vote for the given block.
	VerifyVote(vote *model.Vote) (bool, error)

	// VerifyQC checks the validity of a QC for the given block.
	VerifyQC(qc *model.QuorumCertificate) (bool, error)
}
