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

// Pruner allows components external to HotStuff to keep their un-finalized
// block tree in sync with HotStuff.
type Pruner interface {

	// Prune removes all un-finalized blocks that have become invalidated with
	// the newly finalized block.
	//
	// The set of invalidated blocks is determined as follows:
	// Given the newly finalized block F, for every un-finalized block B, if
	// B's parent is an ancestor of F but B is not an ancestor of F, then B
	// and all of B's children are invalidated.
	Prune(finalizedID flow.Identifier)
}
