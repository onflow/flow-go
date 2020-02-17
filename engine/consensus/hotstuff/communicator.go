package hotstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

// Communicator defines how the HotStuff core algorithm communicates with other
// actors participating in the consensus process.
type Communicator interface {

	// SendVote sends the given vote to the given node.
	SendVote(vote *hotstuff.Vote, to flow.Identifier) error

	// BroadcastProposal sends the given block proposal to all nodes
	// participating in the consensus process.
	BroadcastProposal(proposal *hotstuff.Proposal) error
}
