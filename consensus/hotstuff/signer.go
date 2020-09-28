package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// SignerVerifier can sign and verify HotStuff entities.
type SignerVerifier interface {
	Signer
	Verifier
}

// Signer is responsible for creating votes, proposals and QC's for a given block.
type Signer interface {
	// CreateProposal creates a proposal for the given block.
	CreateProposal(block *model.Block) (*model.Proposal, error)

	// CreateVote creates a vote for the given block.
	CreateVote(block *model.Block) (*model.Vote, error)

	// CreateQC creates a QC for the given block.
	CreateQC(votes []*model.Vote) (*flow.QuorumCertificate, error)
}
