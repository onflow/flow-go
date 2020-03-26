package hotstuff

import "github.com/dapperlabs/flow-go/consensus/hotstuff/model"

// Signer defines a component that is able to sign proposals for the core
// HotStuff algorithm. This component must have the private key used to
// stake.
type Signer interface {

	// VoteFor signs a Block and returns the Vote for that Block.
	VoteFor(block *model.Block) (*model.Vote, error)

	// Propose signs a Block and returns the Proposal.
	Propose(block *model.Block) (*model.Proposal, error)
}
