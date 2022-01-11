package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// Validator provides functions to validate QC, proposals and votes.
type Validator interface {

	// ValidateQC checks the validity of a QC for a given block.
	// During normal operations, the following error returns are expected:
	//	* model.InvalidBlockError if the QC is invalid
	ValidateQC(qc *flow.QuorumCertificate, block *model.Block) error

	// ValidateProposal checks the validity of a proposal.
	// During normal operations, the following error returns are expected:
	//	* model.InvalidBlockError if the block is invalid
	ValidateProposal(proposal *model.Proposal) error

	// ValidateVote checks the validity of a vote for a given block and
	// returns the full entity for the voter. During normal operations,
	// the following errors are expected:
	//  * model.InvalidVoteError for invalid votes
	ValidateVote(vote *model.Vote, block *model.Block) (*flow.Identity, error)
}
