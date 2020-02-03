package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Communicator defines the interface between the HotStuff core algorithm and a
// network layer that handles delivering messages to other nodes participating
// in the consensus process.
type Communicator interface {

	// SendVote sends the given vote to the given node.
	SendVote(vote *types.Vote, to flow.Identifier) error

	// BroadcastProposal sends the given block proposal to all nodes
	// participating in the consensus process.
	BroadcastProposal(proposal *types.BlockProposal) error
}
