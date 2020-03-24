package hotstuff

import (
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

// Signer defines a component that is able to sign proposals for the core
// HotStuff algorithm. This component must have the private key used to
// stake.
type Signer interface {

	// VoteFor signs a Block and returns the Vote for that Block
	VoteFor(block *hotstuff.Block) (*hotstuff.Vote, error)

	// Propose signs a Block and returns the Proposal
	Propose(block *hotstuff.Block) (*hotstuff.Proposal, error)
}
