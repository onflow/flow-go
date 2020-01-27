package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

// NetworkSender defines the interface between the HotStuff core algorithm and a
// networking layer that handles delivering messages to other nodes
// participating in the consensus process.
type NetworkSender interface {

	// SendVote sends the given vote to the given node.
	SendVote(vote *types.Vote, to flow.Identifier) error

	// BroadcastProposal sends the given block proposal to all nodes
	// participating in the consensus process.
	BroadcastProposal(proposal *types.BlockProposal) error
}
