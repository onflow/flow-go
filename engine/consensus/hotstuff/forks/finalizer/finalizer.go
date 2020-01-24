package finalizer

import (
	"bytes"
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/finalizer/forrest"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// Finalizer implements HotStuff finalization logic
type Finalizer struct {
	notifier  notifications.Distributor
	mainChain forrest.LeveledForrest

	// LockedBlockQC is the QC that POINTS TO the the most recently locked block
	LockedBlockQC *types.QuorumCertificate

	// lastFinalizedBlockQC is the QC that POINTS TO the most recently finalized locked block
	LastFinalizedBlockQC *types.QuorumCertificate
}

func New(rootBlock *types.BlockProposal, rootQc *types.QuorumCertificate, notifier notifications.Distributor) (*Finalizer, error) {
	if !bytes.Equal(rootQc.BlockMRH, rootBlock.BlockMRH()) || (rootQc.View != rootBlock.View()) {
		return nil, &types.ErrorConfiguration{Msg: "rootQc must be for rootBlock"}
	}

	fnlzr := Finalizer{
		notifier:             notifier,
		mainChain:            *forrest.NewLeveledForrest(),
		LockedBlockQC:        rootQc,
		LastFinalizedBlockQC: rootQc,
	}

	return &fnlzr, nil
}

func (r *Finalizer) IsKnownBlock(blockID []byte, blockView uint64) bool {
	return r.mainChain.HasVertex(blockID, blockView)
}

func (r *Finalizer) GetBlocksForView(view uint64) []*types.BlockProposal {
	vertexIterator := r.mainChain.GetVerticesAtLevel(view)
	l := make([]*types.BlockProposal, 0, 1) // in the vast majority of cases, there will only be one proposal for a particular view
	for vertexIterator.HasNext() {
		v := vertexIterator.NextVertex().(*BlockContainer)
		l = append(l, v.Block())
	}
	return l
}

func (r *Finalizer) GetBlock(blockID []byte, view uint64) (*types.BlockProposal, bool) {
	blockContainer, hasBlock := r.mainChain.GetVertex(blockID, view)
	if !hasBlock {
		return nil, false
	}
	return blockContainer.(*BlockContainer).Block(), true
}

// IsProcessingNeeded performs basic checks whether or not block needs processing
// only considering the block's height and Hash
// Returns false if any of the following conditions applies
//  * block is for already finalized views
//  * known block
func (r *Finalizer) IsProcessingNeeded(blockID []byte, blockView uint64) bool {
	if blockView <= r.LastFinalizedBlockQC.View || r.IsKnownBlock(blockID, blockView) {
		return false
	}
	return true
}

// IsSafeBlock returns true if block is safe to vote for
// (according to the definition in https://arxiv.org/abs/1803.05069v6).
// Returns false for unknown blocks.
func (r *Finalizer) IsSafeBlock(block *types.BlockProposal) bool {
	blockContainer, hasBlock := r.mainChain.GetVertex(block.BlockMRH(), block.View())
	if !hasBlock {
		return false
	}
	return r.isSafeBlock(blockContainer.(*BlockContainer))
}

// isSafeBlock should only be called on blocks that are connected to the main chain.
// Function returns true iff block is safe to vote for block.
func (r *Finalizer) isSafeBlock(block *BlockContainer) bool {
	// According to the paper, a block is considered a safe block if
	//  * it extends from locked block (safety rule),
	//  * or the view of the parent block is higher than the view number of locked block (liveness rule).
	// The two rules can be boiled down to the following:
	// 1. If block.QC.View is higher than locked view, it definitely is a safe block.
	// 2. If block.QC.View is lower than locked view, it definitely is not a safe block.
	// 3. If block.QC.View equals to locked view: parent must be the locked block.
	qc := block.QC()
	if qc.View > r.LockedBlockQC.View {
		return true
	}
	if (qc.View == r.LockedBlockQC.View) && bytes.Equal(qc.BlockMRH, r.LockedBlockQC.BlockMRH) {
		return true
	}
	return false
}

// ProcessBlock adds `block` to the consensus state.
// Calling this method with previously-processed blocks leaves the consensus state invariant
// (though, it will potentially cause some duplicate processing).
// CAUTION: method assumes that `block` is well-formed (i.e. all fields are set)
func (r *Finalizer) ProcessBlock(block *types.BlockProposal) error {
	if !r.IsProcessingNeeded(block.BlockMRH(), block.View()) {
		return nil
	}
	bc := &BlockContainer{block: block}
	parent, isParentKnown := r.mainChain.GetVertex(block.QC().BlockMRH, block.QC().View)
	if !isParentKnown { //parent not in mainchain
		return &types.ErrorMissingBlock{
			View:    block.QC().View,
			BlockID: block.QC().BlockMRH,
		}
	}
	// parent in mainchain:
	err := r.addToMainChain(bc, parent.(*BlockContainer))
	if err != nil {
		return fmt.Errorf("could not process block: %w", err)
	}
	return nil
}

// addToMainChain adds the block to the main chain and updates consensus state.
// Calling this method with previously-processed blocks leaves the consensus state invariant.
// UNSAFE: assumes that the block is properly formatted and all signatures and QCs are valid
func (r *Finalizer) addToMainChain(blockContainer, mainChainParent *BlockContainer) error {
	err := r.setMainChainProperties(blockContainer, mainChainParent)
	if err != nil {
		return err
	}
	r.mainChain.AddVertex(blockContainer)
	r.updateLockedQc(blockContainer)
	r.updateFinalizedBlockQc(blockContainer)
	r.notifier.OnBlockIncorporated(blockContainer.Block())
	return nil
}

// setMainChainProperties defines all fields in the BlockContainer
// Enforces:
//  * Block's ViewNumber is strictly monotonously increasing
// Prerequisites:
//  * parent must point to correct parent (no check performed!)
func (r *Finalizer) setMainChainProperties(block, parent *BlockContainer) error {
	if block.View() <= parent.View() {
		return &types.ErrorInvalidBlock{
			View:    block.View(),
			BlockID: block.ID(),
			Msg:     fmt.Sprintf("block's view (%d) is SMALLER than its parent's view (%d)", block.View(), parent.View()),
		}
	}
	block.twoChainHead = parent.OneChainHead()
	block.threeChainHead = parent.TwoChainHead()
	return nil
}

// updateLockedBlock updates `LockedBlockQC`
// We use the locking rule from 'Event-driven HotStuff Protocol' where the condition is:
//
// * Consider the set S of all blocks that have a INDIRECT 2-chain on top of it
//
// * The 'Locked Block' is the block in S with the _highest view number_ (newest);
//   LockedBlockQC should be a QC POINTING TO this block
//
// Calling this method with previously-processed blocks leaves consensus state invariant.
func (r *Finalizer) updateLockedQc(block *BlockContainer) {
	if block.TwoChainHead().View <= r.LockedBlockQC.View {
		return
	}
	r.LockedBlockQC = block.TwoChainHead() // update qc to newer block with any 2-chain on top of it
}

// updateFinalizedBlockQc updates `lastFinalizedBlockQC`
// We use the locking rule from 'Event-driven HotStuff Protocol' where the condition is:
//
// * Consider the set S of all blocks that have a DIRECT 2-chain on top of it PLUS any 1-chain
//
// * The 'Last finalized Block' is the block in S with the _highest view number_ (newest);
//   lastFinalizedBlockQC should a QC POINTING TO this block
//
// Calling this method with previously-processed blocks leaves consensus state invariant.
func (r *Finalizer) updateFinalizedBlockQc(block *BlockContainer) {
	// Note: when adding blocks to mainchain, we enforce that Block's ViewNumber is strictly monotonously
	// increasing (method setMainChainProperties). We denote:
	//  * a DIRECT 1-chain as '<-'
	//  * a general 1-chain as '<~' (direct or indirect)
	// The rule from 'Event-driven HotStuff' for finalizing block b is
	//     b <- b' <- b'' <~ b*     (aka a DIRECT 2-chain PLUS any 1-chain)
	// where b* is the input block to this method.
	// Hence, we can finalize b, if and only the viewNumber of b'' is exactly 2 higher than the view of b
	b := block.ThreeChainHead() // note that b is actually not the block itself here but rather the QC pointing to it
	if block.OneChainHead().View == b.View+2 {
		r.finalizeUpToBlock(b)
	}
}

// finalizeUpToBlock finalizes all blocks up to (and including) the block pointed to by `blockQC`.
// Finalization starts with the child of `LastFinalizedBlockQC` (explicitly checked);
// and calls OnFinalizedBlock on the newly finalized blocks in the respective order
func (r *Finalizer) finalizeUpToBlock(blockQC *types.QuorumCertificate) {
	if blockQC.View <= r.LastFinalizedBlockQC.View {
		// Sanity check: the previously last Finalized Block must be an ancestor of `block`
		if !bytes.Equal(r.LastFinalizedBlockQC.BlockMRH, blockQC.BlockMRH) {
			// this should never happen unless the finalization logic in `updateFinalizedBlockQc` is broken
			panic("Internal Error: About to finalize a block which CONFLICTS with last finalized block!")
		}
		return
	}
	// get Block
	blockVertex, _ := r.mainChain.GetVertex(blockQC.BlockMRH, blockQC.View)
	blockContainer := blockVertex.(*BlockContainer)

	// finalize Parent, i.e. the block pointed to by the block's QC
	r.finalizeUpToBlock(blockContainer.QC())

	// finalize block itself
	r.LastFinalizedBlockQC = blockQC
	r.notifier.OnFinalizedBlock(blockContainer.Block())
}
