package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// HotStuff defines the interface to the core HotStuff algorithm. It includes
// a method to start the event loop, and channels to submit proposals and votes
// received from the network.
//
// TODO Define assumptions that the chain compliance layer (ie the user of this
// TODO interface) must satisfy. ie submit blocks in order, etc.
type HotStuff interface {

	// Start starts the HotStuff event loop. It blocks until stopped, or until
	// a fatal error occurs.
	Start() error

	// Proposals returns a write-only channel for submitting new block
	// proposals to the HotStuff core algorithm.
	Proposals() chan<- *types.BlockProposal

	// Votes returns a write-only channel for submitting new votes to the
	// HotStuff core algorithm.
	Votes() chan<- *types.Vote
}

// New sets up and instantiates an instance of the HotStuff core algorithm.
// TODO
func New(signer Signer, network Network, consumer notifications.Consumer) (HotStuff, error) {
	panic("TODO")
}
