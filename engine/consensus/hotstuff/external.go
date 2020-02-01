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

// Builder defines the interface between the HotStuff core algorithm and a
// payload validation layer.
//
// The payload validation layer is responsible for gathering items for
// inclusion in blocks, tracking payloads in un-finalized blocks, and enforcing
// application-specific payload validation rules.
type Builder interface {

	// BuildOn generates a new payload that is valid with respect to the parent
	// being built upon and returns the hash of the generated payload.

	// TODO decide between 2 following:

	// BuildOn1 just generate the payload and return the hash. Store the hash
	// to payload mapping. When the block is eventually broadcasted via
	// NetworkSender, get the payload via block.payloadID and store the full
	// block.
	BuildOn1(parentID flow.Identifier) flow.Identifier

	// BuildOn2 accepts a closure so that storing the full block can be completed
	// all in one go here.
	//
	// create function creates the full block including payload hash, signs it
	// and returns the full block. Builder can then store the block.
	BuildOn2(parentID flow.Identifier, create func(payloadID flow.Identifier) *types.Block)
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
