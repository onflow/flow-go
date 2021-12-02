package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// Signer is responsible for creating votes, proposals for a given block.
type Signer interface {
	// CreateProposal creates a proposal for the given block.
	CreateProposal(block *model.Block) (*model.Proposal, error)

	// CreateVote creates a vote for the given block.
	CreateVote(block *model.Block) (*model.Vote, error)
}
