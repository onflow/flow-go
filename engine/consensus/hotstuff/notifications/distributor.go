package notifications

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// Distributor consumes notifications outbound events produced by HotStuff and its components.
// Outbound events are all events that are potentially relevant to the larger node
// in which HotStuff (or Forks) is running. The notifications are emitted in the order
// in which the HotStuff algorithm makes the respective steps.
//
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type Distributor interface {

	// OnEnteringView notifications are produced by the EventHandler when it enters a new view.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnEnteringView(uint64)

	// OnSkippedAhead notifications are produced by PaceMaker when it decides to skip over one or more view numbers.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnSkippedAhead(uint64)

	// OnStartingTimeout notifications are produced by PaceMaker. Such a notification indicates that the
	// PaceMaker is now waiting for the system to (receive and) process blocks or votes.
	// The specific timeout type is contained in the TimerInfo.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnStartingTimeout(*types.TimerInfo)

	// OnReachedTimeout notifications are produced by PaceMaker. Such a notification indicates that the
	// PaceMaker's timeout was processed by the system.
	// The specific timeout type is contained in the TimerInfo.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnReachedTimeout(*types.TimerInfo)

	// OnQcIncorporated consumes the notifications are produced by ForkChoice
	// whenever a quorum certificate is incorporated into the consensus state.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnQcIncorporated(*types.QuorumCertificate)

	// OnForkChoiceGenerated notifications are produced by ForkChoice whenever a fork choice is generated.
	// The arguments specify the view (first argument) of the block which is to be built and the
	// quorum certificate (second argument) that is supposed to be in the block.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnForkChoiceGenerated(uint64, *types.QuorumCertificate)

	// OnBlockIncorporated notifications are produced by the Finalization Logic
	// whenever a block is incorporated into the consensus state.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnBlockIncorporated(*types.BlockProposal)

	// OnFinalizedBlock notifications are produced by the Finalization Logic whenever a block has been finalized.
	// They are emitted in the order the blocks are finalized.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnFinalizedBlock(*types.BlockProposal)

	// OnDoubleProposeDetected notifications are produced by the Finalization Logic
	// whenever a double block proposal (equivocation) was detected.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnDoubleProposeDetected(*types.BlockProposal, *types.BlockProposal)
}
