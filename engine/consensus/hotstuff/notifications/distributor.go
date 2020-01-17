package notifications

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

// Distributor consumes notifications outbound events produced by HotStuff and its components.
// Outbound events are all events that are potentially relevant to the larger node
// in which HotStuff (or Forks) is running.
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type Distributor interface {
	OnEnteringView(uint64)
	OnSkippedAhead(uint64)

	OnStartingBlockTimeout(uint64)
	OnReachedBlockTimeout(uint64)
	OnStartingVotesTimeout(uint64)
	OnReachedVotesTimeout(uint64)

	OnBlockIncorporated(proposal *types.BlockProposal)
	OnFinalizedBlock(*types.BlockProposal)
	OnDoubleProposeDetected(*types.BlockProposal, *types.BlockProposal)
}

// SkippedViewConsumer consumes notifications of type `OnSkippedAhead`
// which are produced by PaceMaker when it decides to skip over one or more view numbers.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type SkippedAheadConsumer interface {
	// OnSkippedAhead specifies the new view number PaceMaker moves to
	// when it is skipping entire views.
	OnSkippedAhead(newViewNumber uint64)
}

// EnteringViewConsumer consumes notifications of type `OnEnteringView`,
// which are produced by the EventLoop when it enters a new view.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type EnteringViewConsumer interface {
	OnEnteringView(viewNumber uint64)
}

// StartingBlockTimeoutConsumer consumes notifications of type `OnStartingBlockTimeout`,
// which are produced by PaceMaker.
// It indicates that the PaceMaker is now waiting for the system to (receive) and process
// the block for the current view.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type StartingBlockTimeoutConsumer interface {
	OnStartingBlockTimeout(viewNumber uint64)
}

// ReachedBlockTimeoutConsumer consumes notifications of type `OnReachedBlockTimeout`,
// which are produced by PaceMaker.
// It indicates that the PaceMaker's timeout event was processed by the system.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type ReachedBlockTimeoutConsumer interface {
	OnReachedBlockTimeout(viewNumber uint64)
}

// StartingVoteTimeoutConsumer consumes notifications of type `OnStartingVotesTimeout`,
// which are produced by PaceMaker.
// It indicates that the PaceMaker is now waiting for the system to (receive) and process
// votes block for the current view.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type StartingVotesTimeoutConsumer interface {
	OnStartingVotesTimeout(viewNumber uint64)
}

// ReachedVotesConsumer consumes notifications of type `OnReachedVotesTimeout`,
// which are produced by PaceMaker.
// It indicates that the PaceMaker is now waiting for the system to (receive) and process
// votes block for the current view.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type ReachedVotesTimeoutConsumer interface {
	OnReachedVotesTimeout(viewNumber uint64)
}

// BlockIncorporatedConsumer consumes notifications of type `OnBlockIncorporated`,
// which are produced by Forks. It indicates that Forks has incorporated the referenced block
// into the consensus state.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type BlockIncorporatedConsumer interface {
	OnBlockIncorporated(*types.BlockProposal)
}

// FinalizedConsumer consumes notifications of type `OnFinalizedBlock`,
// which are produced by Forks. It indicates that Forks has finalized the referenced block.
// Forks will emit the events in the strict order in which blocks are finalized
// (i.e. with strictly increasing height and view)
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type FinalizedConsumer interface {
	OnFinalizedBlock(*types.BlockProposal)
}

// DoubleProposalConsumer consumes notifications of type `OnDoubleProposeDetected`,
// which are produced by Forks. It indicates that Forks has detected a slashable
// double proposal.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type DoubleProposalConsumer interface {
	OnDoubleProposeDetected(*types.BlockProposal, *types.BlockProposal)
}
