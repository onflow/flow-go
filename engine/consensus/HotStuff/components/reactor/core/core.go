package core

import (
	"bytes"

	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/components/reactor/core/events"
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/components/reactor/state/forrest"
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/def"
)

// ReactorCore implements HotStuff finalization logic
type ReactorCore struct {
	eventProcessor events.Processor

	mainChain forrest.LeveledForrest
	cache     forrest.LeveledForrest

	// LockedBlockQC is the QC that POINTS TO the the most recently locked block
	LockedBlockQC *def.QuorumCertificate

	// lastFinalizedBlockQC is the QC that POINTS TO the most recently finalized locked block
	LastFinalizedBlockQC *def.QuorumCertificate

	// LastSafeBlockView is the view number of the last safe block
	LastSafeBlockView uint64
}

func New(rootBlock *def.Block, rootQc *def.QuorumCertificate, eventProcessor events.Processor) *ReactorCore {
	if rootBlock == nil {
		panic("finalizedRootBlock cannot be nil")
	}
	if eventProcessor == nil {
		panic("eventProcessor cannot be nil")
	}
	if !bytes.Equal(rootQc.BlockMRH, rootBlock.BlockMRH) || (rootQc.View != rootBlock.View) {
		panic("rootQc must be for rootBlock")
	}

	return &ReactorCore{
		eventProcessor:       eventProcessor,
		mainChain:            *forrest.NewLeveledForrest(),
		cache:                *forrest.NewLeveledForrest(),
		LockedBlockQC:        rootQc,
		LastFinalizedBlockQC: rootQc,
		LastSafeBlockView:    rootBlock.View,
	}
}

func (r *ReactorCore) IsKnownBlock(blockMRH []byte, blockView uint64) bool {
	return r.mainChain.HasVertex(blockMRH, blockView) || r.cache.HasVertex(blockMRH, blockView)
}

// IsProcessingNeeded performs basic checks whether or not block needs processing
// only considering the block's height and Hash
// Returns false if any of the following conditions applies
//  * block is for already finalized views
//  * known block
func (r *ReactorCore) IsProcessingNeeded(blockMRH []byte, blockView uint64) bool {
	if blockView <= r.LastFinalizedBlockQC.View || r.IsKnownBlock(blockMRH, blockView) {
		return false
	}
	return true
}

// isSafeBlock should only be called on blocks that are connected to the main chain.
// Function returns true iff block is safe to vote for block.
func (r *ReactorCore) isSafeBlock(block *BlockContainer) bool {
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
// ToDo when adding block to mainchain: check COMPLIANCE with rules for chain evolution (otherwise leave in cache)
func (r *ReactorCore) ProcessBlock(block *def.Block) {
	if !r.IsProcessingNeeded(block.BlockMRH, block.View) {
		// ToDo: we can probably skip this check as
		//  - check is most likely performed in higher-level code
		//  - even if not checked, LeveledForrest handles repeated additions
		//    or additions ob blocks below pruning level gracefully
		return
	}
	bc := &BlockContainer{block: block}
	mainChainParent, hasParentInMainchain := r.mainChain.GetVertex(block.QC.BlockMRH, block.QC.View)
	if !hasParentInMainchain { //if parent not in mainchain: add block to cache and return
		r.cache.AddVertex(bc)
		return
	}

	// parent in mainchain:
	r.addToMainChain(bc, mainChainParent.(*BlockContainer))
	r.mergeBlocksFromCache(bc)
}

// mergeBlocksFromCache adds add descendants of block that are currently in the cache to the main chain.
// block itself is not added, as it is a newly received block that should not be in the main chain.
// Calling this method with previously-processed blocks leaves the consensus state invariant.
func (r *ReactorCore) mergeBlocksFromCache(block *BlockContainer) {
	// ToDo: [Optimization]
	//  (1) move business logic for iterating over all descendants to LeveledForrest as it can be executed more efficiently there.
	//  (2) as cache and main chain are distinct data structures, we could even retrieve the subtree from the cache in one go routine.
	//      and add the vertices to the mainChain in the main thread
	childrenIterator := r.cache.GetChildren(block.Hash(), block.View())
	for childrenIterator.HasNext() {
		// Vertex interface is implemented by VertexMock POINTER!
		// Hence, the concrete type is *VertexMock
		child := childrenIterator.NextVertex().(*BlockContainer)
		r.addToMainChain(child, block)
		r.mergeBlocksFromCache(child)
	}
}

// addToMainChain adds the block to the main chain and updates consensus state.
// UNSAFE: assumes that the block is properly formatted and all signatures and QCs are valid
// Calling this method with previously-processed blocks leaves the consensus state invariant.
func (r *ReactorCore) addToMainChain(blockContainer, mainChainParent *BlockContainer) {
	r.setMainChainProperties(blockContainer, mainChainParent)
	if (blockContainer.View() > r.LastSafeBlockView) && r.isSafeBlock(blockContainer) {
		r.LastSafeBlockView = blockContainer.View()
		defer r.eventProcessor.OnSafeBlock(blockContainer.Block()) // executing the hook should take place after complete state update
	}
	r.mainChain.AddVertex(blockContainer)
	r.updateLockedQc(blockContainer)
	r.updateFinalisedBlockQc(blockContainer)
	r.eventProcessor.OnIncorporatedBlock(blockContainer.Block())
}

// setMainChainProperties defines all fields in the BlockContainer
// Enforces:
//  * Block's ViewNumber is strictly monotonously increasing
// Prerequisites:
//  * parent must point to correct parent (no check performed!)
func (r *ReactorCore) setMainChainProperties(block, parent *BlockContainer) {
	if block.View() <= parent.View() {
		panic("View of a block must be LARGER than its parent")
	}
	block.twoChainHead = parent.OneChainHead()
	block.threeChainHead = parent.TwoChainHead()
}

// updateLockedBlock updates `LockedBlockQC`
// Currently, the method implements 'Chained HotStuff Protocol' where the condition is:
//
// * Consider the set S of all blocks that have a DIRECT 2-chain on top of it
//
// * The 'Locked Block' is the block in S with the _highest view number_ (newest);
//   LockedBlockQC should a QC POINTING TO this block
//
// Calling this method with previously-processed blocks leaves consensus state invariant.
func (r *ReactorCore) updateLockedQc(block *BlockContainer) {
	// r.LockedBlockQC is called 'lockedQC' in 'Chained HotStuff Protocol' https://arxiv.org/abs/1803.05069v6
	twoChainQc := block.TwoChainHead()
	if twoChainQc.View <= r.LockedBlockQC.View {
		return
	}
	// For now, we are implementing the 'Chained HotStuff Protocol' rule which requires a DIRECT 2-chain
	// Note: when adding blocks to mainchain, we enforce that Block's ViewNumber is strictly monotonously
	// increasing (method setMainChainProperties). Hence, the current block forms direct 2-chain, if and only if
	// its viewNumber is exactly 2 higher than the head of the two chain (pointing backwards)
	if block.View() == twoChainQc.View+2 {
		// ToDo [optimization]: update also with an _indirect_ 2-chain, i.e. remove if-clause
		r.LockedBlockQC = twoChainQc
	}
}

// updateFinalisedBlockQc updates `lastFinalizedBlockQC`
// Currently, the method implements 'Chained HotStuff Protocol' where the condition is:
//
// * Consider the set S of all blocks that have a DIRECT 3-chain on top of it
//
// * The 'Last finalized Block' is the block in S with the _highest view number_ (newest);
//   lastFinalizedBlockQC should a QC POINTING TO this block
//
// Calling this method with previously-processed blocks leaves consensus state invariant.
func (r *ReactorCore) updateFinalisedBlockQc(block *BlockContainer) {
	threeChainQc := block.ThreeChainHead()
	if threeChainQc.View <= r.LockedBlockQC.View {
		return
	}
	// For now, we are implementing the 'Chained HotStuff Protocol' rule which requires a DIRECT 3-chain
	// Note: when adding blocks to mainchain, we enforce that Block's ViewNumber is strictly monotonously
	// increasing (method setMainChainProperties). Hence, the current block forms direct 3-chain, if and only if
	// its viewNumber is exactly 3 higher than the head of the three chain (pointing backwards)
	if block.View() == threeChainQc.View+3 {
		r.finalizeUpToBlock(threeChainQc)
	}
}

// finalizeUpToBlock finalizes all block up to (and including) the block pointed to by `blockQC`.
// Finalization starts with the child of `LastFinalizedBlockQC` (explicitly checked);
// and calls OnFinalizedBlock
func (r *ReactorCore) finalizeUpToBlock(blockQC *def.QuorumCertificate) {
	if blockQC.View <= r.LastFinalizedBlockQC.View {
		// Sanity check: the previously last Finalized Block must be an ancestor of `block`
		if !bytes.Equal(r.LastFinalizedBlockQC.BlockMRH, blockQC.Hash) {
			panic("Internal Error: About to finalize a block which CONFLICTS with last finalized block!")
		}
		return
	}
	// get Block
	blockVertex, _ := r.mainChain.GetVertex(blockQC.Hash, blockQC.View)
	blockContainer := blockVertex.(*BlockContainer)

	// finalize Parent, i.e. the block pointed to by the block's QC
	r.finalizeUpToBlock(blockContainer.QC())

	// finalize block itself
	r.LastFinalizedBlockQC = blockQC
	r.eventProcessor.OnFinalizedBlock(blockContainer.Block())
}
