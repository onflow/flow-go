package pruner

import (
	"go.uber.org/atomic"
)

// BlockProcessedProducer is used by components to notify the pruner when it completes processing a block.
// The pruner uses this information to determine the safe lower bound to use when calculating the prune height.
type BlockProcessedProducer struct {
	highestCompleteHeight *atomic.Uint64
}

// OnBlockProcessed notifies the pruner that the producer has completed processing for a block.
func (p *BlockProcessedProducer) OnBlockProcessed(height uint64) {
	p.highestCompleteHeight.Store(height)
}

// TODO: may need a byID version for systems that operate on non-finalized blocks (e.g. execution)
