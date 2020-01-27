package notifications

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

// SkippedAheadConsumer consumes notifications of type `OnSkippedAhead`
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

type QcIncorporatedConsumer interface {
	OnQcIncorporated(*types.QuorumCertificate)
}

type ForkChoiceGeneratedConsumer interface {
	OnForkChoiceGenerated(uint64, *types.QuorumCertificate)
}

type BlockIncorporatedConsumer interface {
	OnBlockIncorporated(*types.BlockProposal)
}

type FinalizedBlockConsumer interface {
	OnFinalizedBlock(*types.BlockProposal)
}

type DoubleProposeDetectedConsumer interface {
	OnDoubleProposeDetected(*types.BlockProposal, *types.BlockProposal)
}
