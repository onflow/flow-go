package forks

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/forkchoice"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/utils"
	"github.com/juju/loggo"
)

var ConsensusLogger loggo.Logger

// Vessle implements the hotstuff.Reactor API
type Vessle struct {
	finalizationLogic *finalizer.ReactorCore
	forkchoice        forkchoice.ForkChoice
}

func (v *Vessle) GetBlocksForView(view uint64) []*types.BlockProposal {
	return v.finalizationLogic.GetBlocksForView(view)
}

func (v *Vessle) GetBlock(view uint64, blockMRH []byte) (*types.BlockProposal, bool) {
	return v.finalizationLogic.GetBlock(blockMRH, view)
}

func (v *Vessle) FinalizedView() uint64 {
	return v.finalizationLogic.LastFinalizedBlockQC.View
}

func (v *Vessle) FinalizedBlock() *types.BlockProposal {
	qc := v.finalizationLogic.LastFinalizedBlockQC // QC that POINTS TO the most recently finalized locked block
	block, _ := v.GetBlock(qc.View, qc.BlockMRH)   // there is _always_ a finalized block
	return block
}

func (v *Vessle) IsSafeNode(block *types.BlockProposal) bool {
	return v.finalizationLogic.IsSafeBlock(block)
}

func (v *Vessle) IsKnownBlock(blockMRH []byte, blockView uint64) bool {
	return v.finalizationLogic.IsKnownBlock(blockMRH, blockView)
}

func (v *Vessle) IsProcessingNeeded(blockMRH []byte, blockView uint64) bool {
	return v.finalizationLogic.IsProcessingNeeded(blockMRH, blockView)
}

func (v *Vessle) AddBlock(block *types.BlockProposal) {
	v.forkchoice.ProcessBlock(block)
}

func (v *Vessle) AddQC(qc *types.QuorumCertificate) {
	v.forkchoice.ProcessQc(qc)
}

func (v *Vessle) MakeForkChoice(viewNumber uint64) *types.QuorumCertificate {
	return v.forkchoice.MakeForkChoice(viewNumber)
}

func NewVessle(finalizer *finalizer.ReactorCore, forkchoice forkchoice.ForkChoice) *Vessle {
	utils.EnsureNotNil(finalizer, "Finalization Logic")
	utils.EnsureNotNil(forkchoice, "ForkChoice")
	return &Vessle{
		finalizationLogic: finalizer,
		forkchoice:        forkchoice,
	}
}
