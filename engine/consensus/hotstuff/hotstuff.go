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

	// SubmitProposal submits a new block proposal to the HotStuff event loop.
	// This method blocks until the proposal is accepted to the event queue.
	// TODO assumptions about proposals
	SubmitProposal(*types.BlockProposal)

	// SubmitVote submits a new vote to the HotStuff event loop.
	// This method blocks until the vote is accepted to the event queue.
	// TODO assumptions about votes
	SubmitVote(*types.Vote)
}

// New sets up and instantiates an instance of the HotStuff core algorithm.
// TODO
func New(signer Signer, network NetworkSender, consumer notifications.Consumer) (HotStuff, error) {
	panic("TODO")
}
