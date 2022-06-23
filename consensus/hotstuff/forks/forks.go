package forks

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// Forks is a tiny wrapper over Finalizer which implements hotstuff.Forks interface
// and performs state lookups and safe insertion of blocks
type Forks struct {
	finalizer Finalizer
}

var _ hotstuff.Forks = (*Forks)(nil)

// New creates a Forks instance
func New(finalizer Finalizer) *Forks {
	return &Forks{
		finalizer: finalizer,
	}
}

// GetBlocksForView returns all the blocks for a certain view.
func (f *Forks) GetBlocksForView(view uint64) []*model.Block {
	return f.finalizer.GetBlocksForView(view)
}

// GetBlock returns the block for the given block ID
func (f *Forks) GetBlock(id flow.Identifier) (*model.Block, bool) {
	return f.finalizer.GetBlock(id)
}

// FinalizedBlock returns the latest finalized block
func (f *Forks) FinalizedBlock() *model.Block {
	return f.finalizer.FinalizedBlock()
}

// FinalizedView returns the view of the latest finalized block
func (f *Forks) FinalizedView() uint64 {
	return f.finalizer.FinalizedBlock().View
}

// AddBlock passes the block to the finalizer for finalization
func (f *Forks) AddBlock(block *model.Block) error {
	if err := f.finalizer.VerifyBlock(block); err != nil {
		// technically, this not strictly required. However, we leave this as a sanity check for now
		return fmt.Errorf("cannot add invalid block to Forks: %w", err)
	}
	err := f.finalizer.AddBlock(block)
	if err != nil {
		return fmt.Errorf("error storing block in Forks: %w", err)
	}

	return nil
}
