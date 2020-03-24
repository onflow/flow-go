package notifications

import (
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

// FinalizationConsumer consumes outbound notifications produced by the finalization logic.
// Notifications represent finalization-specific state changes which are potentially relevant
// to the larger node. The notifications are emitted in the order in which the
// finalization algorithm makes the respective steps.
//
// Implementations must:
//   * be concurrency safe
//   * be non-blocking
//   * handle repetition of the same events (with some processing overhead).
type FinalizationConsumer interface {

	// OnBlockIncorporated notifications are produced by the Finalization Logic
	// whenever a block is incorporated into the consensus state.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnBlockIncorporated(*hotstuff.Block)

	// OnFinalizedBlock notifications are produced by the Finalization Logic whenever
	// a block has been finalized. They are emitted in the order the blocks are finalized.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnFinalizedBlock(*hotstuff.Block)

	// OnDoubleProposeDetected notifications are produced by the Finalization Logic
	// whenever a double block proposal (equivocation) was detected.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnDoubleProposeDetected(*hotstuff.Block, *hotstuff.Block)
}

// Consumer consumes outbound notifications produced by HotStuff and its components.
// Notifications are consensus-internal state changes which are potentially relevant to
// the larger node in which HotStuff is running. The notifications are emitted
// in the order in which the HotStuff algorithm makes the respective steps.
//
// Implementations must:
//   * be concurrency safe
//   * be non-blocking
//   * handle repetition of the same events (with some processing overhead).
type Consumer interface {
	FinalizationConsumer

	// OnEnteringView notifications are produced by the EventHandler when it enters a new view.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnEnteringView(viewNumber uint64)

	// OnSkippedAhead notifications are produced by PaceMaker when it decides to skip over one or more view numbers.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnSkippedAhead(viewNumber uint64)

	// OnStartingTimeout notifications are produced by PaceMaker. Such a notification indicates that the
	// PaceMaker is now waiting for the system to (receive and) process blocks or votes.
	// The specific timeout type is contained in the TimerInfo.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnStartingTimeout(*hotstuff.TimerInfo)

	// OnReachedTimeout notifications are produced by PaceMaker. Such a notification indicates that the
	// PaceMaker's timeout was processed by the system.
	// The specific timeout type is contained in the TimerInfo.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnReachedTimeout(*hotstuff.TimerInfo)

	// OnQcIncorporated consumes the notifications are produced by ForkChoice
	// whenever a quorum certificate is incorporated into the consensus state.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnQcIncorporated(*hotstuff.QuorumCertificate)

	// OnForkChoiceGenerated notifications are produced by ForkChoice whenever a fork choice is generated.
	// The arguments specify the view (first argument) of the block which is to be built and the
	// quorum certificate (second argument) that is supposed to be in the block.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnForkChoiceGenerated(uint64, *hotstuff.QuorumCertificate)

	// OnDoubleVotingDetected notifications are produced by the Vote Aggregation logic
	// whenever a double voting (same voter voting for different blocks at the same view) was detected.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnDoubleVotingDetected(*hotstuff.Vote, *hotstuff.Vote)

	// OnInvalidVoteDetected notifications are produced by the Vote Aggregation logic
	// whenever an invalid vote was detected.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnInvalidVoteDetected(*hotstuff.Vote)
}
