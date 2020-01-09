package events

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// Processor consumes events produced by reactor.core
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type Processor interface {
	OnMissingBlock(hash []byte, view uint64)
	OnBlockIncorporated(proposal *types.BlockProposal)
	OnSafeBlock(*types.BlockProposal)
	OnFinalizedBlock(*types.BlockProposal)

	// Detected slashing conditions:
	OnDoubleProposeDetected(*types.BlockProposal, *types.BlockProposal)
}

// MissingBlockConsumer consumes the following type of event produced by reactor.core:
// whenever a block is found missing, the `OnMissingBlock` event is triggered
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type MissingBlockConsumer interface {
	OnMissingBlock([]byte, uint64)
}

// BlockIncorporatedConsumer consumes the following type of event produced by reactor.core:
// whenever a block has been incorporated into the consensus state (mainchain), the `OnBlockIncorporated` event is triggered
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type BlockIncorporatedConsumer interface {
	OnBlockIncorporated(*types.BlockProposal)
}

// SafeBlockConsumer consumes the following type of event produced by reactor.core:
// whenever a block is found to be safe, the `OnSafeBlock` event is triggered
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type SafeBlockConsumer interface {
	OnSafeBlock(*types.BlockProposal)
}

// FinalizedConsumer consumes the following type of event produced by reactor.core:
// whenever a block has been finalized, the `OnFinalizedBlock` event is triggered
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type FinalizedConsumer interface {
	OnFinalizedBlock(*types.BlockProposal)
}

// DoubleProposalConsumer consumes the following type of event produced by reactor.core:
// whenever a double block proposal (equivocation) was detected, the `DoubleProposeDetected` event is triggered
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type DoubleProposalConsumer interface {
	OnDoubleProposeDetected(*types.BlockProposal, *types.BlockProposal)
}
