package hotstuff

import (
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

// Signer defines a component that is able to sign proposals for the core
// HotStuff algorithm. This component must have the private key used to
// stake.
type Signer interface {
	VoteFor(block *hotstuff.Block) (*hotstuff.Vote, error)
	Propose(block *hotstuff.Block) (*hotstuff.Proposal, error)
}
