package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// SigValidator only validates the cryptographic signature of an entity (Vote, Block or QC).
// It builds the message corresponding to the entity, and uses the cryptographic verifier to validate
// the signature. The verifier used does not have any knowledge about the entity.
type SigValidator interface {
	// ValidateVote validates the cryptographic signature of a vote.
	// It returns:
	//   - nil if the vote's signature is valid
	//   - model.InvalidVoteError if the vote's signature is invalid
	//   - other error if there is an exception
	ValidateVote(vote *model.Vote) error

	// ValidateBlock validates the cryptographic signature of a block.
	// It returns:
	//   - nil if the blocks's signature is valid
	//   - model.InvalidBlockError if the block's signature is invalid
	//   - other error if there is an exception
	ValidateBlock(block *model.Proposal) error
}

type QCValidator interface {
	// ValidateQC validates the cryptographic signature of a QC.
	// It returns:
	//   - nil if the QC's signature is valid
	//   - model.InvalidQCError if the block's signature is invalid
	//   - other error if there is an exception
	ValidateQC(qc *flow.QuorumCertificate) error
}
