package notifications

import (
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

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

// StartingTimeoutConsumer consumes notifications of type `OnStartingTimeout`,
// which are produced by PaceMaker. Such a notification indicates that the
// PaceMaker is now waiting for the system to (receive and) process blocks or votes.
// The specific timeout type is contained in the TimerInfo.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type StartingTimeoutConsumer interface {
	OnStartingTimeout(timerInfo *hotstuff.TimerInfo)
}

// ReachedTimeoutConsumer consumes notifications of type `OnReachedTimeout`,
// which are produced by PaceMaker. Such a notification indicates that the
// PaceMaker's timeout was processed by the system.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type ReachedTimeoutConsumer interface {
	OnReachedTimeout(timeout *hotstuff.TimerInfo)
}

// QcIncorporatedConsumer consumes notifications of type `OnQcIncorporated`,
// which are produced by ForkChoice. Such a notification indicates that
// a quorum certificate is incorporated into the consensus state.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type QcIncorporatedConsumer interface {
	OnQcIncorporated(*hotstuff.QuorumCertificate)
}

// ForkChoiceGeneratedConsumer consumes notifications of type `OnForkChoiceGenerated`,
// which are produced by ForkChoice. Such a notification indicates that a fork choice has
// been generated. The notification contains the view of the block which was build and
// the quorum certificate that was used to build the block.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type ForkChoiceGeneratedConsumer interface {
	OnForkChoiceGenerated(uint64, *hotstuff.QuorumCertificate)
}

// BlockIncorporatedConsumer consumes notifications of type `OnBlockIncorporated`,
// which are produced by the Finalization Logic. Such a notification indicates
// that a new block was incorporated into the consensus state.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type BlockIncorporatedConsumer interface {
	OnBlockIncorporated(*hotstuff.Block)
}

// FinalizedBlockConsumer consumes notifications of type `OnFinalizedBlock`,
// which are produced by the Finalization Logic. Such a notification indicates
// that a block has been finalized.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type FinalizedBlockConsumer interface {
	OnFinalizedBlock(*hotstuff.Block)
}

// DoubleProposeDetectedConsumer consumes notifications of type `OnDoubleProposeDetected`,
// which are produced by the Finalization Logic. Such a notification indicates
// that a double block proposal (equivocation) was detected.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type DoubleProposeDetectedConsumer interface {
	OnDoubleProposeDetected(*hotstuff.Block, *hotstuff.Block)
}

// OnDoubleVotingDetected notifications are produced by the Vote Aggregation logic
// whenever a double voting (same voter voting for different blocks at the same view) was detected.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type DoubleVotingDetectedConsumer interface {
	OnDoubleVotingDetected(*hotstuff.Vote, *hotstuff.Vote)
}

// OnInvalidVoteDetected notifications are produced by the Vote Aggregation logic
// whenever an invalid vote was detected.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type InvalidVoteDetectedConsumer interface {
	OnInvalidVoteDetected(*hotstuff.Vote)
}
