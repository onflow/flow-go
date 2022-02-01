package forks

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// Forks implements the hotstuff.Reactor API
type Forks struct {
	finalizer  Finalizer
	forkchoice ForkChoice
}

var _ hotstuff.Forks = (*Forks)(nil)

// New creates a Forks instance
func New(finalizer Finalizer, forkchoice ForkChoice) *Forks {
	return &Forks{
		finalizer:  finalizer,
		forkchoice: forkchoice,
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

// IsSafeBlock returns whether a block is safe to vote for.
func (f *Forks) IsSafeBlock(block *model.Block) bool {
	if err := f.finalizer.VerifyBlock(block); err != nil {
		return false
	}
	return f.finalizer.IsSafeBlock(block)
}

// AddBlock passes the block to the finalizer for finalization and
// gives the QC to forkchoice for updating the preferred parent block
func (f *Forks) AddBlock(block *model.Block) error {
	if err := f.finalizer.VerifyBlock(block); err != nil {
		// technically, this not strictly required. However, we leave this as a sanity check for now
		return fmt.Errorf("cannot add invalid block to Forks: %w", err)
	}
	err := f.finalizer.AddBlock(block)
	if err != nil {
		return fmt.Errorf("error storing block in Forks: %w", err)
	}

	// We only process the block's QC if the block's view is larger than the last finalized block.
	// By ignoring hte qc's of block's at or below the finalized view, we allow the genesis block
	// to have a nil QC.
	if block.View <= f.finalizer.FinalizedBlock().View {
		return nil
	}
	return f.AddQC(block.QC)
}

// MakeForkChoice returns the block to build new block proposal from for the current view.
// the QC is the QC that points to that block.
func (f *Forks) MakeForkChoice(curView uint64) (*flow.QuorumCertificate, *model.Block, error) {
	return f.forkchoice.MakeForkChoice(curView)
}

// AddQC gives the QC to the forkchoice for updating the preferred parent block
func (f *Forks) AddQC(qc *flow.QuorumCertificate) error {
	return f.forkchoice.AddQC(qc) // forkchoice ensures that block referenced by qc is known
}
