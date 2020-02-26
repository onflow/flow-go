package finalizer

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
