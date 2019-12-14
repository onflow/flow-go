package events

import (
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/def"
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/defConAct"
)

// Processor consumes events produced by reactor.core
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type Processor interface {
	OnMissingBlock(hash []byte, view uint64)
	OnIncorporatedBlock(*def.Block)
	OnSafeBlock(*def.Block)
	OnFinalizedBlock(*def.Block)

	// Detected slashing conditions:
	OnDoubleVoteDetected(*defConAct.Vote, *defConAct.Vote)
	OnDoubleProposeDetected(*def.Block, *def.Block)
}

// MissingBlockConsumer consumes the following type of event produced by reactor.core:
// whenever a block is found missing, the `OnMissingBlock` event is triggered
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type MissingBlockConsumer interface {
	OnMissingBlock([]byte, uint64)
}

// IncorporatedBlockConsumer consumes the following type of event produced by reactor.core:
// whenever a block has been incorporated into the consensus state (mainchain), the `OnIncorporatedBlock` event is triggered
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type IncorporatedBlockConsumer interface {
	OnIncorporatedBlock(*def.Block)
}

// SafeBlockConsumer consumes the following type of event produced by reactor.core:
// whenever a block is found to be safe, the `OnSafeBlock` event is triggered
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type SafeBlockConsumer interface {
	OnSafeBlock(*def.Block)
}

// FinalizedConsumer consumes the following type of event produced by reactor.core:
// whenever a block has been finalized, the `OnFinalizedBlock` event is triggered
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type FinalizedConsumer interface {
	OnFinalizedBlock(*def.Block)
}

// DoubleVoteConsumer consumes the following type of event produced by reactor.core:
// whenever a double voting (equivocation) was detected, the `DoubleVoteDetected` event is triggered
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type DoubleVoteConsumer interface {
	OnDoubleVoteDetected(*defConAct.Vote, *defConAct.Vote)
}

// DoubleProposalConsumer consumes the following type of event produced by reactor.core:
// whenever a double block proposal (equivocation) was detected, the `DoubleProposeDetected` event is triggered
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type DoubleProposalConsumer interface {
	OnDoubleProposeDetected(*def.Block, *def.Block)
}
