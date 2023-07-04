package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// Validator provides functions to validate QC, proposals and votes.
type Validator interface {

	// ValidateQC checks the validity of a QC.
	// During normal operations, the following error returns are expected:
	//  * model.InvalidQCError if the QC is invalid
	//  * model.ErrViewForUnknownEpoch if the QC refers unknown epoch
	ValidateQC(qc *flow.QuorumCertificate) error

	// ValidateTC checks the validity of a TC.
	// During normal operations, the following error returns are expected:
	//  * model.InvalidTCError if the TC is invalid
	//  * model.ErrViewForUnknownEpoch if the TC refers unknown epoch
	ValidateTC(tc *flow.TimeoutCertificate) error

	// ValidateProposal checks the validity of a proposal.
	// During normal operations, the following error returns are expected:
	//  * model.InvalidProposalError if the block is invalid
	//  * model.ErrViewForUnknownEpoch if the proposal refers unknown epoch
	ValidateProposal(proposal *model.Proposal) error

	// ValidateVote checks the validity of a vote.
	// Returns the full entity for the voter. During normal operations,
	// the following errors are expected:
	//  * model.InvalidVoteError for invalid votes
	//  * model.ErrViewForUnknownEpoch if the vote refers unknown epoch
	ValidateVote(vote *model.Vote) (*flow.IdentitySkeleton, error)
}
