package forks

import (
	"github.com/dapperlabs/flow-go/engine/consensus/eventdriven/modules/utils"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/forkchoice"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/juju/loggo"
)

var ConsensusLogger loggo.Logger

var defaultForks hotstuff.Forks = Forks{}


// Vessle implements the hotstuff.Reactor API
type Forks struct {
	finalizer *finalizer.Finalizer
	forkchoice        ForkChoice
}

func (f Forks) GetBlocksForView(view uint64) []*types.BlockProposal {
	return f.finalizer.GetBlocksForView(view)
}

func (f Forks) GetBlock(id []byte) (*types.BlockProposal, bool) {
	return f.finalizer.GetBlock(id)
}

func (f Forks) FinalizedView() uint64 {
	return f.finalizer.LastFinalizedBlockQC.View
}

func (f Forks) FinalizedBlock() *types.BlockProposal {
	f.finalizer.GetBlock(f.finalizer.LastFinalizedBlockQC.BlockMRH)

	return
}

func (f Forks) LockedBlockQc() *types.QuorumCertificate {
	return f.finalizer.LastLockedBlockQC
}

func (f Forks) LockedBlockQC() *types.QuorumCertificate {
	return f.finalizer.LastFinalizedBlockQC
}

// LastLockedBlockQC is the QC that POINTS TO the the most recently locked block
 *types.QuorumCertificate

// lastFinalizedBlockQC is the QC that POINTS TO the most recently finalized locked block

func (f Forks) IsSafeBlock(block *types.BlockProposal) (bool, error) {
	panic("implement me")
}

func (f Forks) AddBlock(block *types.BlockProposal) error {
	panic("implement me")
}

func (f Forks) AddQC(qc *types.QuorumCertificate) error {
	panic("implement me")
}

func (f Forks) MakeForkChoice(curView uint64) (*types.QuorumCertificate, error) {
	panic("implement me")
}




func (v *Vessle) GetBlocksForView(view uint64) []*types.BlockProposal {
	return v.finalizationLogic.GetBlocksForView(view)
}

func (v *Vessle) GetBlock(view uint64, blockID []byte) (*types.BlockProposal, bool) {
	return v.finalizationLogic.GetBlock(blockID, view)
}

func (v *Vessle) FinalizedView() uint64 {
	return v.finalizationLogic.LastFinalizedBlockQC.View
}

func (v *Vessle) FinalizedBlock() *types.BlockProposal {
	qc := v.finalizationLogic.LastFinalizedBlockQC // QC that POINTS TO the most recently finalized locked block
	block, _ := v.GetBlock(qc.View, qc.BlockID)   // there is _always_ a finalized block
	return block
}

func (v *Vessle) IsSafeNode(block *types.BlockProposal) bool {
	return v.finalizationLogic.IsSafeBlock(block)
}

func (v *Vessle) IsKnownBlock(blockID []byte, blockView uint64) bool {
	return v.finalizationLogic.IsKnownBlock(blockID, blockView)
}

func (v *Vessle) IsProcessingNeeded(blockID []byte, blockView uint64) bool {
	return v.finalizationLogic.IsProcessingNeeded(blockID, blockView)
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
