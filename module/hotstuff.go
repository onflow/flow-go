package module

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// HotStuff defines the interface for the core HotStuff algorithm. It includes
// a method to start the event loop, and utilities to submit block proposals
// received from other replicas.
//
// TODO:
//
//	HotStuff interface could extend HotStuffFollower. Thereby, we can
//	utilize the optimized catchup mode from the follower also for the
//	consensus participant.
type HotStuff interface {
	ReadyDoneAware
	Startable

	// SubmitProposal submits a new block proposal to the HotStuff event loop.
	// This method blocks until the proposal is accepted to the event queue.
	//
	// Block proposals must be submitted in order and only if they extend a
	// block already known to HotStuff core.
	SubmitProposal(proposal *model.Proposal)
}

// HotStuffFollower is run by non-consensus nodes to observe the block chain
// and make local determination about block finalization. While the process of
// reaching consensus (incl. guaranteeing its safety and liveness) is very intricate,
// the criteria to confirm that consensus has been reached are relatively straight
// forward. Each non-consensus node can simply observe the blockchain and determine
// locally which blocks have been finalized without requiring additional information
// from the consensus nodes.
//
// In contrast to an active HotStuff participant, the HotStuffFollower does not validate
// block payloads. This greatly reduces the amount of CPU and memory that it consumes.
// Essentially, the consensus participants exhaustively verify the entire block including
// the payload and only vote for the block if it is valid. The consensus committee
// aggregates votes from a supermajority of participants to a Quorum Certificate [QC].
// Thereby, it is guaranteed that only valid blocks get certified (receive a QC).
// By only consuming certified blocks, the HotStuffFollower can be sure of their
// correctness and omit the heavy payload verification.
// There is no disbenefit for nodes to wait for a QC (included in child blocks), because
// all nodes other than consensus generally require the Source Of Randomness included in
// QCs to process the block in the first place.
//
// The central purpose of the HotStuffFollower is to inform other components within the
// node about finalization of blocks.
//
// Notes:
//   - HotStuffFollower internally prunes blocks below the last finalized view.
//   - HotStuffFollower does not handle disconnected blocks. For each input block,
//     we require that the parent was previously added (unless the parent's view
//     is _below_ the latest finalized view).
type HotStuffFollower interface {
	ReadyDoneAware
	Startable

	// AddCertifiedBlock appends the given certified block to the tree of pending
	// blocks and updates the latest finalized block (if finalization progressed).
	// Unless the parent is below the pruning threshold (latest finalized view), we
	// require that the parent has previously been added.
	//
	// Notes:
	//  - Under normal operations, this method is non-blocking. The follower internally
	//    queues incoming blocks and processes them in its own worker routine. However,
	//    when the inbound queue is full, we block until there is space in the queue. This
	//    behaviour is intentional, because we cannot drop blocks (otherwise, we would
	//    cause disconnected blocks). Instead we simply block the compliance layer to
	//    avoid any pathological edge cases.
	//  - Blocks whose views are below the latest finalized view are dropped.
	//  - Inputs are idempotent (repetitions are no-ops).
	AddCertifiedBlock(certifiedBlock *model.CertifiedBlock)
}
