package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// Signer defines a component that is able to sign proposals for the core
// HotStuff algorithm. This component must have the private key used to
// stake.
type Signer interface {
	VoteFor(block *types.Block) (*types.Vote, error)
	Propose(block *types.Block) (*types.Proposal, error)
}
