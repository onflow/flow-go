package module

import (
	"github.com/onflow/flow-go/model/flow"
)

// HotStuff defines the interface to the core HotStuff algorithm. It includes
// a method to start the event loop, and utilities to submit block proposals
//  received from other replicas.
type HotStuff interface {
	ReadyDoneAware
	Startable

	// SubmitProposal submits a new block proposal to the HotStuff event loop.
	// This method blocks until the proposal is accepted to the event queue.
	//
	// Block proposals must be submitted in order and only if they extend a
	// block already known to HotStuff core.
	SubmitProposal(proposal *flow.Header, parentView uint64)
}

// HotStuffFollower is run by non-consensus nodes to observe the block chain
// and make local determination about block finalization. While the process of
// reaching consensus (while guaranteeing its safety and liveness) is very intricate,
// the criteria to confirm that consensus has been reached are relatively straight
// forward. Each non-consensus node can simply observe the blockchain and determine
// locally which blocks have been finalized without requiring additional information
// from the consensus nodes.
//
// Specifically, the HotStuffFollower informs other components within the node
// about finalization of blocks. It consumes block proposals broadcasted
// by the consensus node, verifies the block header and locally evaluates
// the finalization rules.
//
// Notes:
//   * HotStuffFollower does not handle disconnected blocks. Each block's parent must
//	   have been previously processed by the HotStuffFollower.
//   * HotStuffFollower internally prunes blocks below the last finalized view.
//     When receiving a block proposal, it might not have the proposal's parent anymore.
//     Nevertheless, HotStuffFollower needs the parent's view, which must be supplied
//     in addition to the proposal.
type HotStuffFollower interface {
	ReadyDoneAware

	// SubmitProposal feeds a new block proposal into the HotStuffFollower.
	// This method blocks until the proposal is accepted to the event queue.
	//
	// Block proposals must be submitted in order, i.e. a proposal's parent must
	// have been previously processed by the HotStuffFollower.
	SubmitProposal(proposal *flow.Header, parentView uint64)
}
