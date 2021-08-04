package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// SigValidator only validate a message's signature part.
// It transforms the vote or block into a message, and uses the verifier, which doesn't have
// the knowledge of vote/block, to verify the signature.
type SigValidator interface {
	// return nil if the vote's signature is valid
	// return model.InvalidVoteError if the vote's signature is invalid
	// return other error if there is exception
	ValidateVote(vote *model.Vote) error

	// return nil if the block's signature is valid
	// return model.InvalidBlockError if the block's signature is invalid
	// return other error if there is exception
	ValidateBlock(block *model.Proposal) error

	// return nil if the QC's signature is valid
	// return model.InvalidBlockError if the QC's signature is invalid
	// return other error if there is exception
	ValidateQC(qc *flow.QuorumCertificate) error
}
