package finalizer

import (
	"bytes"
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
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

type threeChain struct {
	block *BlockContainer
	oneChainQC *types.QuorumCertificate
	oneChainBlock *BlockContainer
	twoChainQC *types.QuorumCertificate
	twoChainBlock *BlockContainer
	threeChainQC *types.QuorumCertificate
	threeChainBlock *BlockContainer
}

// ############ :-) ############# //


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

	// If rootBlock has view > 0, we can already pre-prune the levelled Forrest to the view below it.
	// Thereby, the levelled forrest won't event store older (unnecessary) blocks
	if rootBlock.View() > 0 {
		err := fnlzr.mainChain.PruneAtLevel(rootBlock.View()-1)
		if err != nil {
			return nil, fmt.Errorf("internal leveled forrest error: %w", err)
		}
	}
	// verify and add root block to leveled forrest
	err := fnlzr.VerifyBlock(rootBlock)
	if err != nil {
		return nil, fmt.Errorf("invalid root block: %w", err)
	}
	fnlzr.mainChain.AddVertex(&BlockContainer{block: rootBlock})
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
// UNVALIDATED: expects block to pass Finalizer.VerifyBlock(block)
func (r *Finalizer) IsKnownBlock(block *types.BlockProposal) bool {
	_, hasBlock := r.mainChain.GetVertex(block.BlockMRH())
	return hasBlock
}

// isProcessingNeeded performs basic checks whether or not block needs processing
// only considering the block's height and Hash
// Returns false if any of the following conditions applies
//  * block view is _below_ the most recently finalized block
//  * known block
// UNVALIDATED: expects block to pass Finalizer.VerifyBlock(block)
func (r *Finalizer) IsProcessingNeeded(block *types.BlockProposal) bool {
	if block.View() < r.LastFinalizedBlockQC.View || r.IsKnownBlock(block) {
		return false
	}
	return true
}

// IsSafeBlock returns true if block is safe to vote for
// (according to the definition in https://arxiv.org/abs/1803.05069v6).
// NO MODIFICATION of consensus state (read only)
// UNVALIDATED: expects block to pass Finalizer.VerifyBlock(block)
func (r *Finalizer) IsSafeBlock(block *types.BlockProposal) (bool, error) {
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
// UNVALIDATED: expects block to pass Finalizer.VerifyBlock(block)
func (r *Finalizer) AddBlock(block *types.BlockProposal) error {
	if !r.IsProcessingNeeded(block) {
		return nil
	}
	blockContainer := &BlockContainer{block: block}
	r.mainChain.AddVertex(blockContainer)
	r.updateConsensusState(blockContainer)
	return nil
}

// getParentBlockAndQC parent from mainChain.
// returns parent BlockContainer and in addition the qc pointing to the parent
// (i.e. the blockContainer.QC())
// UNVALIDATED: expects block to pass Finalizer.VerifyBlock(block)
func (r *Finalizer) getParentBlockAndQC(blockContainer *BlockContainer) (*BlockContainer, *types.QuorumCertificate, error) {
	oneChainVertex, oneChainBlockKnown := r.mainChain.GetVertex(blockContainer.QC().BlockMRH)
	if !oneChainBlockKnown {
		return nil, nil, &ErrorPruned3Chain{blockContainer}
	}
	return oneChainVertex.(*BlockContainer), blockContainer.QC(), nil
}

// getThreeChain returns the three chain or a ErrorPruned3Chain sentinel error
// to indicate that the 3-chain from blockContainer is (partially) pruned
func (r *Finalizer) getThreeChain(blockContainer *BlockContainer) (*threeChain, error) {
	threeChain := threeChain{block:blockContainer}

	var err error
	threeChain.oneChainBlock, threeChain.oneChainQC, err = r.getParentBlockAndQC(blockContainer)
	if err != nil {
		return nil, err
	}
	threeChain.twoChainBlock, threeChain.twoChainQC, err = r.getParentBlockAndQC(threeChain.oneChainBlock)
	if err != nil {
		return nil, err
	}
	threeChain.threeChainBlock, threeChain.threeChainQC, err = r.getParentBlockAndQC(threeChain.twoChainBlock)
	if err != nil {
		return nil, err
	}
	return &threeChain, nil
}

// updateConsensusState updates consensus state.
// Calling this method with previously-processed blocks leaves the consensus state invariant.
// UNVALIDATED: assumes that relevant block properties are consistent with previous blocks
func (r *Finalizer) updateConsensusState(blockContainer *BlockContainer) error {
	threeChain, err := r.getThreeChain(blockContainer)
	if err != nil {
		switch err.(type) {
		case *ErrorPruned3Chain:
			return nil // for finalization, we ignore all blocks which do not have a full un-pruned 3-chain
		default:
			fmt.Errorf("unexpected error while updarting consensus state: %w", err)
		}
	}
	r.updateLockedQc(threeChain)
	r.updateFinalizedBlockQc(threeChain)
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
func (r *Finalizer) updateLockedQc(threeChain *threeChain) {
	if threeChain.twoChainQC.View <= r.LastLockedBlockQC.View {
		return
	}
	r.LastLockedBlockQC = threeChain.twoChainQC // update qc to newer block with any 2-chain on top of it
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
func (r *Finalizer) updateFinalizedBlockQc(threeChain *threeChain) {
	// Note: when adding blocks to mainchain, we enforce that Block's ViewNumber is strictly monotonously
	// increasing (method setMainChainProperties). We denote:
	//  * a DIRECT 1-chain as '<-'
	//  * a general 1-chain as '<~' (direct or indirect)
	// The rule from 'Event-driven HotStuff' for finalizing block b is
	//     b <- b' <- b'' <~ b*     (aka a DIRECT 2-chain PLUS any 1-chain)
	// where b* is the input block to this method.
	// Hence, we can finalize b, if and only the viewNumber of b'' is exactly 2 higher than the view of b
	b := threeChain.threeChainQC // note that b is actually not the block itself here but rather the QC pointing to it
	if threeChain.oneChainQC.View == b.View+2 {
		r.finalizeUpToBlock(b)
	}
}



// ############ :-) ############# //x


// finalizeUpToBlock finalizes all blocks up to (and including) the block pointed to by `blockQC`.
// Finalization starts with the child of `LastFinalizedBlockQC` (explicitly checked);
// and calls OnFinalizedBlock on the newly finalized blocks in the respective order
func (r *Finalizer) finalizeUpToBlock(blockQC *types.QuorumCertificate) (*BlockContainer, error) {
	if blockQC.View <= r.LastFinalizedBlockQC.View {
		// Sanity check: the previously last Finalized Block must be an ancestor of `block`
		if !bytes.Equal(r.LastFinalizedBlockQC.BlockMRH, blockQC.BlockMRH) {
			return nil, &hotstuff.ErrorByzantineSuperminority{fmt.Sprintf(
				"finalizing blocks at conflicting forks: %s and %s",
				string(blockQC.BlockMRH), string(r.LastFinalizedBlockQC.BlockMRH),
			)}
		}
		return r.LastFinalizedBlock, nil
	}
	// Have:
	//   (1) blockQC.View > r.LastFinalizedBlockQC.View => finalizing new block
	// Corollary
	//   (2) blockContainer.View >= 1
	// Explanation: We require that Forks is initialized with a _finalized_ rootBlock,
	// which has view >= 0. Hence, r.LastFinalizedBlockQC.View >= 0, by which (1) implies (2)

	// get Block and finalize everything up to the block's parent
	blockVertex, _ := r.mainChain.GetVertex(blockQC.BlockMRH) // require block to resolve parent
	blockContainer := blockVertex.(*BlockContainer)
	r.finalizeUpToBlock(blockContainer.QC()) // finalize Parent, i.e. the block pointed to by the block's QC

	// finalize block itself:
	r.LastFinalizedBlockQC = blockQC
	r.LastFinalizedBlock = blockContainer
	r.mainChain.PruneAtLevel(blockContainer.View()-1) // cannot underflow as of (2)
	r.notifier.OnFinalizedBlock(blockContainer.Block())
	return blockContainer, nil
}

// hasPrunedView returns if givem view number is already in the pruned rande.
func (r *Finalizer) isPrunedView(view uint64) bool {
	return view < r.mainChain.LowestLevel
}


// VerifyBlock checks block for validity
func (r *Finalizer) VerifyBlock(block *types.BlockProposal) error {
	if r.isPrunedView(block.View()) {
		return nil
	}
	blockContainer := &BlockContainer{block: block}
	err := r.mainChain.VerifyVertex(blockContainer)
	if err != nil {
		fmt.Errorf("invalid block: %w", err)
	}

	// omit checking existence of parent if block at lowest non-pruned view number
	if (block.View() == r.mainChain.LowestLevel) || (block.QC().View < r.mainChain.LowestLevel) {
		return nil
	}
	// for block whose parents are _not_ below the pruning height, we expect the parent to be known.
	if _, isParentKnown := r.mainChain.GetVertex(block.QC().BlockMRH); !isParentKnown { // we are missing the parent
		return &types.ErrorMissingBlock{
			View:    block.QC().View,
			BlockID: block.QC().BlockMRH,
		}
	}
	return nil
}
