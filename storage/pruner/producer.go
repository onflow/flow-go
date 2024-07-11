package pruner

import (
	"go.uber.org/atomic"
)

// BlockProcessedProducer is used by components to notify the pruner when it completes processing a block.
// The pruner uses this information to determine the safe lower bound to use when calculating the prune height.
type BlockProcessedProducer struct {
	highestCompleteHeight *atomic.Uint64
}

// NewBlockProcessedProducer creates a new BlockProcessedProducer.
func NewBlockProcessedProducer() *BlockProcessedProducer {
	return &BlockProcessedProducer{
		highestCompleteHeight: atomic.NewUint64(0),
	}
}

// OnBlockProcessed notifies the pruner that the producer has completed processing for a block.
func (p *BlockProcessedProducer) OnBlockProcessed(height uint64) {
	p.highestCompleteHeight.Store(height)
}
