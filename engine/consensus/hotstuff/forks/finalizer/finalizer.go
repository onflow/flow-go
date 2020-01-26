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

	// LastLockedBlockQC is the QC that POINTS TO the the most recently locked block
	LastLockedBlockQC *types.QuorumCertificate

	// LastFinalizedBlock is the last most recently finalized locked block
	LastFinalizedBlock   *BlockContainer

	// lastFinalizedBlockQC is the QC that POINTS TO the most recently finalized locked block
	LastFinalizedBlockQC *types.QuorumCertificate
}


func New(rootBlock *types.BlockProposal, rootQc *types.QuorumCertificate, notifier notifications.Distributor) (*Finalizer, error) {
	if !bytes.Equal(rootQc.BlockMRH, rootBlock.BlockMRH()) || (rootQc.View != rootBlock.View()) {
		return nil, &types.ErrorConfiguration{Msg: "rootQc must be for rootBlock"}
	}

	rootBlockContainer := &BlockContainer{block: rootBlock}
	fnlzr := Finalizer{
		notifier:             notifier,
		mainChain:            *forrest.NewLeveledForrest(),
		LastFinalizedBlock: rootBlockContainer,
		LastLockedBlockQC:    rootQc,
		LastFinalizedBlockQC: rootQc,
	}
	err := fnlzr.mainChain.AddVertex(rootBlockContainer)
	if err != nil {
		return nil, fmt.Errorf("error initialzing Finalizer: %w", err)
	}
	if rootBlock.View() > 0 {
		// If rootBlock has view > 0, we can already pre-prune the levelled Forrest
		// to the view belo it. Thereby, the levelled forrest
		err = fnlzr.mainChain.PruneAtLevel(rootBlock.View()-1)
	}
	if err != nil {
		return nil, fmt.Errorf("error initialzing Finalizer: %w", err)
	}
	return &fnlzr, nil
}

// GetBlock returns block for given ID
func (r *Finalizer) GetBlock(blockID []byte) (*types.BlockProposal, bool) {
	blockContainer, hasBlock := r.mainChain.GetVertex(blockID)
	if !hasBlock {
		return nil, false
	}
	return blockContainer.(*BlockContainer).Block(), true
}

// GetBlock returns all known blocks for the given
func (r *Finalizer) GetBlocksForView(view uint64) []*types.BlockProposal {
	vertexIterator := r.mainChain.GetVerticesAtLevel(view)
	l := make([]*types.BlockProposal, 0, 1) // in the vast majority of cases, there will only be one proposal for a particular view
	for vertexIterator.HasNext() {
		v := vertexIterator.NextVertex().(*BlockContainer)
		l = append(l, v.Block())
	}
	return l
}

// IsKnownBlock checks whether block is known.
// Can error with ErrorBlockHashCollision.
func (r *Finalizer) IsKnownBlock(block *types.BlockProposal) (bool, error) {
	vertex, exists := r.mainChain.GetVertex(block.BlockMRH())
	if !exists {
		return false, nil
	}
	return r.areBlocksRedundant(block, vertex.(*BlockContainer).block) // errors with ErrorBlockHashCollision
}


// areBlocksRedundant [errors with ErrorBlockHashCollision]
// evaluates whether two blocks carry redundant information from the view-point of the finalizer.
//
// (1) return value (false, nil)
// Two block are NOT REDUNDANT if they have different IDs (Hashes).
//
// (2) return value (true, nil)
// Two blocks ARE REDUNDANT if their respective fields are identical:
// ID (Hash), view number, and quorum Certificate (qc), i.e qc.view and qc.blockID.
// (ignoring any other potential fields the blocks might have.
//
// (3) return value (false, ErrorBlockHashCollision)
// areBlocksRedundant errors if the blocks' IDs are identical but they differ
// in any of the _relevant_ fields (as defined in (2)).
func (r *Finalizer) areBlocksRedundant(block1, block2 *types.BlockProposal) (bool, error) {
	if !bytes.Equal(block1.BlockMRH(), block2.BlockMRH()) {
		return false, nil
	}

	// hashes of blocks are identical => we expect all relevant values to be identical
	if block1.View() != block2.View() { // view number
		return false, &ErrorBlockHashCollision{block1: block1, block2: block2, location: "View"}
	}
	if !bytes.Equal(block1.QC().BlockMRH, block2.QC().BlockMRH) { // qc.blockID
		return false, &ErrorBlockHashCollision{block1: block1, block2: block2, location: "qc.blockID"}
	}
	if block1.QC().View != block2.QC().View { // qc.view
		return false, &ErrorBlockHashCollision{block1: block1, block2: block2, location: "qc.view"}
	}
	// all _relevant_ fields identical
	return true, nil
}

// isProcessingNeeded performs basic checks whether or not block needs processing
// only considering the block's height and Hash
// Returns false if any of the following conditions applies
//  * block is for already finalized views
//  * known block
// Can error with ErrorBlockHashCollision.
func (r *Finalizer) IsProcessingNeeded(block *types.BlockProposal) (bool, error) {
	if block.View() <= r.LastFinalizedBlockQC.View {
		return false, nil
	}
	isKnownBlock, err := r.IsKnownBlock(block) // errors with ErrorBlockHashCollision
	if  err != nil {
		return false, fmt.Errorf("cannot evaluate whether block should be processed: %w", err)
	}
	return !isKnownBlock, nil
}

// IsSafeBlock returns true if block is safe to vote for
// (according to the definition in https://arxiv.org/abs/1803.05069v6).
//
// In the current architecture, the block is stored _before_ evaluating its safety.
// Consequently, IsSafeBlock accepts only known, valid blocks. Should a block be
// unknown (not previously added to Forks) or violate some consistency requirements,
// IsSafeBlock errors. All errors are fatal.
func (r *Finalizer) IsSafeBlock(block *types.BlockProposal) (bool, error) {
	isKnownBlock, err := r.IsKnownBlock(block) // errors with ErrorBlockHashCollision
	if err!= nil {
		return false, fmt.Errorf("cannot evaluate whether block is safe to vote for: %w", err)
	}
	if !isKnownBlock {
		return false, fmt.Errorf("IsSafeBlock only accepts known blocks", err)
	}

	// According to the paper, a block is considered a safe block if
	//  * it extends from locked block (safety rule),
	//  * or the view of the parent block is higher than the view number of locked block (liveness rule).
	// The two rules can be boiled down to the following:
	// 1. If block.QC.View is higher than locked view, it definitely is a safe block.
	// 2. If block.QC.View is lower than locked view, it definitely is not a safe block.
	// 3. If block.QC.View equals to locked view: parent must be the locked block.
	qc := block.QC()
	if qc.View > r.LastLockedBlockQC.View {
		return true, nil
	}
	if (qc.View == r.LastLockedBlockQC.View) && bytes.Equal(qc.BlockMRH, r.LastLockedBlockQC.BlockMRH) {
		return true, nil
	}
	return false, nil
}


// ProcessBlock adds `block` to the consensus state.
// Calling this method with previously-processed blocks leaves the consensus state invariant
// (though, it will potentially cause some duplicate processing).
// CAUTION: method assumes that `block` is well-formed (i.e. all fields are set)
func (r *Finalizer) AddBlock(block *types.BlockProposal) error {
	if !(block.View() > block.QC().View) {
		return &types.ErrorInvalidBlock{
			View:    block.View(),
			BlockID: block.BlockMRH(),
			Msg:     fmt.Sprintf("block's view (%d) is SMALLER than its qc.view (%d)", block.View(), parent.View()),
		}
	}
	isProcessingNeeded, err := r.IsProcessingNeeded(block) // errors with ErrorBlockHashCollision
	if err != nil {
		return fmt.Errorf("cannot process block: %w", err)
	}
	if !isProcessingNeeded {
		return nil
	}

	if block.QC().View < r.LastFinalizedBlock.View() {
		// the parent of block is already pruned
		blockContainer := &BlockContainer{block: block}
		err := r.mainChain.AddVertex(blockContainer)
		if err != nil {
			return fmt.Errorf("error adding block: %w", err)
		}
		return nil
	}
	// block.QC().View > r.LastFinalizedBlock.View() Hence, block's parent should be in mainChain

	blockContainer, err := r.wrapAsBlockContainer(block)
	if err != nil {
		return fmt.Errorf("cannot add invalid block: %w", err)
	}
	r.mainChain.AddVertex(blockContainer) // store blockContainer in mainChain
	if err != nil {
		return fmt.Errorf("cannot add store: %w", err)
	}

	r.updateLockedQc(blockContainer)
	r.updateFinalizedBlockQc(blockContainer)
	r.notifier.OnBlockIncorporated(blockContainer.Block())
	return nil
}

// wrapAsBlockContainer constructs a BlockContainer for holding block.
// Errors with ErrorMissingBlock if the parent is unknown.
// Errors with ErrorInvalidBlock is the block is inconsistent with the current consensus state.
func (r *Finalizer) wrapAsBlockContainer(block *types.BlockProposal) (*BlockContainer, error) {
	blockContainer := &BlockContainer{block: block}
	parentContainer, err := r.getParentContainer(block)
	if err != nil {
		fmt.Errorf("cannot get parent container for block: %w", err)
	}
	blockContainer.twoChainHead = parentContainer.OneChainHead()
	blockContainer.threeChainHead = parentContainer.TwoChainHead()
	return blockContainer, nil
}

// retrieves parent from mainChain by hash and enforces
// Errors with
// * ErrorMissingBlock is parent is unknown
// * ErrorInvalidBlock block's view is not greater than its parent's view.
// * ErrorInvalidBlock if qc points to known block with same hash,
//   but it has mismatching view number
// Erroring on these conditions enforces:
//  * Block's ViewNumber is strictly monotonously increasing
//  * QC is consistent with parent in both variables: ID (Hash) and View
func (r *Finalizer) getParentContainer(block *types.BlockProposal) (*BlockContainer, error) {
	parentVertex, isParentKnown := r.mainChain.GetVertex(block.QC().BlockMRH)
	if !isParentKnown { //parent not in mainchain
		return nil, &types.ErrorMissingBlock{
			View:    block.QC().View,
			BlockID: block.QC().BlockMRH,
		}
	}

	parent := parentVertex.(*BlockContainer)
	if !(block.View() > parent.View()) {
		return nil, &types.ErrorInvalidBlock{
			View:    block.View(),
			BlockID: block.BlockMRH(),
			Msg:     fmt.Sprintf("block's view (%d) is SMALLER than its parent's view (%d)", block.View(), parent.View()),
		}
	}
	if parent.View() != block.QC().View {
		return nil, &types.ErrorInvalidBlock{
			View:    block.View(),
			BlockID: block.BlockMRH(),
			Msg:     "qc points to known block with same hash, but it has mismatching view number",
		}
	}
	return parent, nil
}

// updateConsensusState updates consensus state.
// Calling this method with previously-processed blocks leaves the consensus state invariant.
// UNSAFE: assumes that relevant block properties are consistent with previous blocks
func (r *Finalizer) updateConsensusState(blockContainer *BlockContainer) error {
	r.updateLockedQc(blockContainer)
	r.updateFinalizedBlockQc(blockContainer)
	r.notifier.OnBlockIncorporated(blockContainer.Block())
	return nil
}


// updateLockedBlock updates `LastLockedBlockQC`
// We use the locking rule from 'Event-driven HotStuff Protocol' where the condition is:
//
// * Consider the set S of all blocks that have a INDIRECT 2-chain on top of it
//
// * The 'Locked Block' is the block in S with the _highest view number_ (newest);
//   LastLockedBlockQC should be a QC POINTING TO this block
//
// Calling this method with previously-processed blocks leaves consensus state invariant.
func (r *Finalizer) updateLockedQc(block *BlockContainer) {
	if block.TwoChainHead() == nil {
		// <2-chain head> points below pruned view, which implies:
		// <2-chain head>.view < LastFinalizedBlockQC.View <= LastLockedBlockQC.View
		// which implies:
		//    <2-chain head>.view < LastLockedBlockQC.View
		return
	}
	if block.TwoChainHead().View <= r.LastLockedBlockQC.View {
		return
	}
	r.LastLockedBlockQC = block.TwoChainHead() // update qc to newer block with any 2-chain on top of it
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
	if block.ThreeChainHead() == nil {
		// <3-chain head> points below pruned view, which implies
		// that block should not lead to a different block being finalized
		return
	}

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
	blockVertex, _ := r.mainChain.GetVertex(blockQC.BlockMRH)
	blockContainer := blockVertex.(*BlockContainer)

	// finalize Parent, i.e. the block pointed to by the block's QC
	r.finalizeUpToBlock(blockContainer.QC())

	// finalize block itself
	r.LastFinalizedBlockQC = blockQC
	r.LastFinalizedBlock = blockContainer
	r.notifier.OnFinalizedBlock(blockContainer.Block())
}
