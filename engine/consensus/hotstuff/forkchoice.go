package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type ForkChoice struct {
	lockedBlock    *types.BlockProposal
	finalizedBlock *types.BlockProposal
	genericQC      *types.QuorumCertificate
	mainForrest    *LevelledForrest
}

// Queries

func (fc *ForkChoice) GetQCForNextBlock(view uint64) *types.QuorumCertificate {
	panic("TODO")
}

func (fc *ForkChoice) FindBlockForView(view uint64) *types.BlockProposal {
	panic("TODO")
}

func (fc *ForkChoice) FindBlockProposalByViewAndBlockMRH(view uint64, blockMRH types.MRH) *types.BlockProposal {
	panic("TODO")
}

func (fc *ForkChoice) FinalizedView() uint64 {
	panic("TODO")
}

func (fc *ForkChoice) IsSafeNode(block *types.BlockProposal) bool {
	panic("TODO")
}

// return true only if the following conditions are all true
// 1. above the finalized block's View
// 2. block's QC is pointing to a leaf block on the tree
func (fc *ForkChoice) CanIncorperate(bp *types.BlockProposal) bool {
	panic("TODO")
}

// Updates

func (fc *ForkChoice) UpdateValidQC(qc *types.QuorumCertificate) (genericQCUpdated bool, finalizedBlock *types.BlockProposal) {
	panic("TODO")
}

func (fc *ForkChoice) AddNewBlock(bp *types.BlockProposal) (incorperatedBlock *types.BlockProposal) {
	panic("TODO")
}
