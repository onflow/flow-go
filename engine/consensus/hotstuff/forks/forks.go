package forks

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Vessle implements the hotstuff.Reactor API
type Forks struct {
	finalizer  *finalizer.Finalizer
	forkchoice ForkChoice
}

func (f *Forks) GetBlocksForView(view uint64) []*types.BlockProposal {
	return f.finalizer.GetBlocksForView(view)
}
func (f *Forks) GetBlock(id []byte) (*types.BlockProposal, bool) { return f.finalizer.GetBlock(id) }
func (f *Forks) FinalizedView() uint64                           { return f.finalizer.FinalizedBlock().View() }
func (f *Forks) FinalizedBlock() *types.BlockProposal            { return f.finalizer.FinalizedBlock() }

func (f *Forks) IsSafeBlock(block *types.BlockProposal) bool {
	return f.finalizer.IsKnownBlock(block)
}

func (f *Forks) AddBlock(block *types.BlockProposal) error {
	if err := f.finalizer.VerifyBlock(block); err != nil {
		// technically, this not strictly required. However, we leave this as a sanity check for now
		return fmt.Errorf("cannot add invalid block to Forks: %w", err)
	}
	err := f.finalizer.AddBlock(block)
	if err != nil {
		return fmt.Errorf("error storing block in Forks: %w", err)
	}
	if block.View() <= f.finalizer.FinalizedBlock().View() {
		return nil
	}
	return f.AddQC(block.QC())
}

func (f *Forks) MakeForkChoice(curView uint64) (*types.QCBlock, error) {
	return f.forkchoice.MakeForkChoice(curView)
}

func (f *Forks) AddQC(qc *types.QuorumCertificate) error {
	return f.forkchoice.AddQC(qc)
}

func (f *Forks) ensureBlockStored(qc *types.QuorumCertificate) (*types.BlockProposal, error) {
	block, haveBlock := f.finalizer.GetBlock(qc.BlockMRH)
	if !haveBlock {
		return nil, &types.ErrorMissingBlock{View: qc.View, BlockID: qc.BlockMRH}
	}
	if block.View() != qc.View {
		return nil, &types.ErrorInvalidBlock{
			View:    qc.View,
			BlockID: qc.BlockMRH,
			Msg:     fmt.Sprintf("block with this ID has view %d", block.View()),
		}
	}
	return block, nil
}

func (f *Forks) VerifyBlock(header *flow.Header) error {
	// ToDo implement
	panic("convert header to ")
}

func New(finalizer *finalizer.Finalizer, forkchoice ForkChoice) hotstuff.Forks {
	return &Forks{
		finalizer:  finalizer,
		forkchoice: forkchoice,
	}
}
