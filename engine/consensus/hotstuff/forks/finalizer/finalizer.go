package finalizer

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/finalizer/forest"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Finalizer implements HotStuff finalization logic
type Finalizer struct {
	notifier notifications.Distributor
	forest  forest.LevelledForest

	lastLockedBlock   *BlockContainer          // lastLockedBlockQC is the QC that POINTS TO the the most recently locked block
	lastLockedBlockQC *types.QuorumCertificate // lastLockedBlockQC is the QC that POINTS TO the the most recently locked block

	lastFinalizedBlock   *BlockContainer          // lastFinalizedBlock is the last most recently finalized locked block
	lastFinalizedBlockQC *types.QuorumCertificate // lastFinalizedBlockQC is the QC that POINTS TO the most recently finalized locked block
}

type ancestryChain struct {
	block           *BlockContainer
	oneChainQC      *types.QuorumCertificate
	oneChainBlock   *BlockContainer
	twoChainQC      *types.QuorumCertificate
	twoChainBlock   *BlockContainer
	threeChainQC    *types.QuorumCertificate
	threeChainBlock *BlockContainer
}

func New(rootBlock *types.BlockProposal, rootQc *types.QuorumCertificate, notifier notifications.Distributor) (*Finalizer, error) {
	if (rootQc.BlockID != rootBlock.BlockID()) || (rootQc.View != rootBlock.View()) {
		return nil, &types.ErrorConfiguration{Msg: "rootQc must be for rootBlock"}
	}

	rootBlockContainer := &BlockContainer{block: rootBlock}
	fnlzr := Finalizer{
		notifier:             notifier,
		forest:              *forest.NewLevelledForest(),
		lastLockedBlock:      rootBlockContainer,
		lastLockedBlockQC:    rootQc,
		lastFinalizedBlock:   rootBlockContainer,
		lastFinalizedBlockQC: rootQc,
	}

	// If rootBlock has view > 0, we can already pre-prune the levelled forest to the view below it.
	// Thereby, the levelled forest won't event store older (unnecessary) blocks
	if rootBlock.View() > 0 {
		err := fnlzr.forest.PruneAtLevel(rootBlock.View() - 1)
		if err != nil {
			return nil, fmt.Errorf("internal levelled forest error: %w", err)
		}
	}
	// verify and add root block to levelled forest
	err := fnlzr.VerifyBlock(rootBlock)
	if err != nil {
		return nil, fmt.Errorf("invalid root block: %w", err)
	}
	fnlzr.forest.AddVertex(&BlockContainer{block: rootBlock})
	return &fnlzr, nil
}

func (r *Finalizer) LockedBlock() *types.BlockProposal          { return r.lastLockedBlock.Block() }
func (r *Finalizer) LockedBlockQC() *types.QuorumCertificate    { return r.lastLockedBlockQC }
func (r *Finalizer) FinalizedBlock() *types.BlockProposal       { return r.lastFinalizedBlock.Block() }
func (r *Finalizer) FinalizedBlockQC() *types.QuorumCertificate { return r.lastFinalizedBlockQC }

// GetBlock returns block for given ID
func (r *Finalizer) GetBlock(blockID *flow.Identifier) (*types.BlockProposal, bool) {
	blockContainer, hasBlock := r.forest.GetVertex(blockID)
	if !hasBlock {
		return nil, false
	}
	return blockContainer.(*BlockContainer).Block(), true
}

// GetBlock returns all known blocks for the given
func (r *Finalizer) GetBlocksForView(view uint64) []*types.BlockProposal {
	vertexIterator := r.forest.GetVerticesAtLevel(view)
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
	id := block.BlockID()
	_, hasBlock := r.forest.GetVertex(&id)
	return hasBlock
}

// isProcessingNeeded performs basic checks whether or not block needs processing
// only considering the block's height and Hash
// Returns false if any of the following conditions applies
//  * block view is _below_ the most recently finalized block
//  * known block
// UNVALIDATED: expects block to pass Finalizer.VerifyBlock(block)
func (r *Finalizer) IsProcessingNeeded(block *types.BlockProposal) bool {
	if block.View() < r.lastFinalizedBlockQC.View || r.IsKnownBlock(block) {
		return false
	}
	return true
}

// IsSafeBlock returns true if block is safe to vote for
// (according to the definition in https://arxiv.org/abs/1803.05069v6).
// NO MODIFICATION of consensus state (read only)
// UNVALIDATED: expects block to pass Finalizer.VerifyBlock(block)
func (r *Finalizer) IsSafeBlock(block *types.BlockProposal) bool {
	// According to the paper, a block is considered a safe block if
	//  * it extends from locked block (safety rule),
	//  * or the view of the parent block is higher than the view number of locked block (liveness rule).
	// The two rules can be boiled down to the following:
	// 1. If block.QC.View is higher than locked view, it definitely is a safe block.
	// 2. If block.QC.View is lower than locked view, it definitely is not a safe block.
	// 3. If block.QC.View equals to locked view: parent must be the locked block.
	qc := block.QC()
	if qc.View > r.lastLockedBlockQC.View {
		return true
	}
	if (qc.View == r.lastLockedBlockQC.View) && (qc.BlockID == r.lastLockedBlockQC.BlockID) {
		return true
	}
	return false
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
	if err := r.checkForByzantineQC(blockContainer); err != nil {
		return err
	}
	r.checkForDoubleProposal(blockContainer)
	r.forest.AddVertex(blockContainer)
	return r.updateConsensusState(blockContainer)
}

func (r *Finalizer) checkForByzantineQC(block *BlockContainer) error {
	parentBlockID, parentView := block.Parent()
	it := r.forest.GetVerticesAtLevel(parentView)
	for it.HasNext() {
		otherBlock := it.NextVertex() // by construction, must have same view as parentView
		if parentBlockID != otherBlock.VertexID() {
			// * we have just found another block at the same view number as block.qc but with different hash
			// * if this block has a child c, this child will have
			//   c.qc.view = parentView
			//   c.qc.ID != parentBlockID
			// => conflicting qc
			otherChildren := r.forest.GetChildren(otherBlock.VertexID())
			if otherChildren.HasNext() {
				otherChild := otherChildren.NextVertex()
				conflictingQC := otherChild.(*BlockContainer).QC()
				return &hotstuff.ErrorByzantineThresholdExceeded{Evidence: fmt.Sprintf(
					"conflicting QCs at view %d: %v and %v",
					parentView, parentBlockID, conflictingQC.BlockID,
				)}
			}
		}
	}
	return nil
}

// checkForDoubleProposal checks if Block is a doubl;e proposal. In case it is,
// notifications.OnDoubleProposeDetected is triggered
func (r *Finalizer) checkForDoubleProposal(block *BlockContainer) {
	it := r.forest.GetVerticesAtLevel(block.View())
	for it.HasNext() {
		otherVertex := it.NextVertex() // by construction, must have same view as parentView
		if block.ID() != otherVertex.VertexID() {
			r.notifier.OnDoubleProposeDetected(block.Block(), otherVertex.(*BlockContainer).Block())
		}
	}
}

// updateConsensusState updates consensus state.
// Calling this method with previously-processed blocks leaves the consensus state invariant.
// UNVALIDATED: assumes that relevant block properties are consistent with previous blocks
func (r *Finalizer) updateConsensusState(blockContainer *BlockContainer) error {
	ancestryChain, err := r.getThreeChain(blockContainer)
	if err != nil {
		switch err.(type) {
		case *ErrorPruned3Chain:
			return nil // for finalization, we ignore all blocks which do not have a full un-pruned 3-chain
		default:
			return fmt.Errorf("unexpected error while updarting consensus state: %w", err)
		}
	}
	r.updateLockedQc(ancestryChain)
	err = r.updateFinalizedBlockQc(ancestryChain)
	if err != nil {
		return err
	}
	r.notifier.OnBlockIncorporated(blockContainer.Block())
	return nil
}

// getThreeChain returns the three chain or a ErrorPruned3Chain sentinel error
// to indicate that the 3-chain from blockContainer is (partially) pruned
func (r *Finalizer) getThreeChain(blockContainer *BlockContainer) (*ancestryChain, error) {
	ancestryChain := ancestryChain{block: blockContainer}

	var err error
	ancestryChain.oneChainBlock, ancestryChain.oneChainQC, err = r.getParentBlockAndQC(blockContainer)
	if err != nil {
		return nil, err
	}
	ancestryChain.twoChainBlock, ancestryChain.twoChainQC, err = r.getParentBlockAndQC(ancestryChain.oneChainBlock)
	if err != nil {
		return nil, err
	}
	ancestryChain.threeChainBlock, ancestryChain.threeChainQC, err = r.getParentBlockAndQC(ancestryChain.twoChainBlock)
	if err != nil {
		return nil, err
	}
	return &ancestryChain, nil
}

// getParentBlockAndQC parent from forest.
// returns parent BlockContainer and, for convenience, the qc pointing to the parent
// (i.e. blockContainer.QC())
// UNVALIDATED: expects block to pass Finalizer.VerifyBlock(block)
func (r *Finalizer) getParentBlockAndQC(blockContainer *BlockContainer) (*BlockContainer, *types.QuorumCertificate, error) {
	id := blockContainer.QC().BlockID
	parentVertex, parentBlockKnown := r.forest.GetVertex(&id)
	if !parentBlockKnown {
		return nil, nil, &types.ErrorMissingBlock{View: blockContainer.QC().View, BlockID: blockContainer.QC().BlockID}
	}
	return parentVertex.(*BlockContainer), blockContainer.QC(), nil
}

// updateLockedBlock updates `lastLockedBlockQC`
// We use the locking rule from 'Event-driven HotStuff Protocol' where the condition is:
//
// * Consider the set S of all blocks that have a INDIRECT 2-chain on top of it
//
// * The 'Locked Block' is the block in S with the _highest view number_ (newest);
//   lastLockedBlockQC should be a QC POINTING TO this block
//
// Calling this method with previously-processed blocks leaves consensus state invariant.
func (r *Finalizer) updateLockedQc(ancestryChain *ancestryChain) {
	if ancestryChain.twoChainQC.View <= r.lastLockedBlockQC.View {
		return
	}
	// update qc to newer block with any 2-chain on top of it:
	r.lastLockedBlockQC = ancestryChain.twoChainQC
	r.lastFinalizedBlock = ancestryChain.twoChainBlock
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
func (r *Finalizer) updateFinalizedBlockQc(ancestryChain *ancestryChain) error {
	// Note: when adding blocks to mainchain, we enforce that Block's ViewNumber is strictly monotonously
	// increasing (method setMainChainProperties). We denote:
	//  * a DIRECT 1-chain as '<-'
	//  * a general 1-chain as '<~' (direct or indirect)
	// The rule from 'Event-driven HotStuff' for finalizing block b is
	//     b <- b' <- b'' <~ b*     (aka a DIRECT 2-chain PLUS any 1-chain)
	// where b* is the input block to this method.
	// Hence, we can finalize b, if and only the viewNumber of b'' is exactly 2 higher than the view of b
	b := ancestryChain.threeChainQC // note that b is actually not the block itself here but rather the QC pointing to it
	if ancestryChain.oneChainQC.View != b.View+2 {
		return nil
	}
	return r.finalizeUpToBlock(b)
}

// finalizeUpToBlock finalizes all blocks up to (and including) the block pointed to by `blockQC`.
// Finalization starts with the child of `lastFinalizedBlockQC` (explicitly checked);
// and calls OnFinalizedBlock on the newly finalized blocks in the respective order
func (r *Finalizer) finalizeUpToBlock(blockQC *types.QuorumCertificate) error {
	if blockQC.View <= r.lastFinalizedBlockQC.View {
		// Sanity check: the previously last Finalized Block must be an ancestor of `block`
		if r.lastFinalizedBlockQC.BlockID != blockQC.BlockID {
			return &hotstuff.ErrorByzantineThresholdExceeded{Evidence: fmt.Sprintf(
				"finalizing blocks at conflicting forks: %v and %v",
				blockQC.BlockID, r.lastFinalizedBlockQC.BlockID,
			)}
		}
		return nil
	}
	// Have:
	//   (1) blockQC.View > r.lastFinalizedBlockQC.View => finalizing new block
	// Corollary
	//   (2) blockContainer.View >= 1
	// Explanation: We require that Forks is initialized with a _finalized_ rootBlock,
	// which has view >= 0. Hence, r.lastFinalizedBlockQC.View >= 0, by which (1) implies (2)

	// get Block and finalize everything up to the block's parent
	blockVertex, _ := r.forest.GetVertex(&blockQC.BlockID) // require block to resolve parent
	blockContainer := blockVertex.(*BlockContainer)
	err := r.finalizeUpToBlock(blockContainer.QC()) // finalize Parent, i.e. the block pointed to by the block's QC
	if err != nil {
		return err
	}

	// finalize block itself:
	r.lastFinalizedBlockQC = blockQC
	r.lastFinalizedBlock = blockContainer
	err = r.forest.PruneAtLevel(blockContainer.View() - 1) // cannot underflow as of (2)
	if err != nil {
		return fmt.Errorf("pruning levelled forest failed: %w", err)
	}
	r.notifier.OnFinalizedBlock(blockContainer.Block())
	return nil
}

// VerifyBlock checks block for validity
func (r *Finalizer) VerifyBlock(block *types.BlockProposal) error {
	if block.View() < r.forest.LowestLevel {
		return nil
	}
	blockContainer := &BlockContainer{block: block}
	err := r.forest.VerifyVertex(blockContainer)
	if err != nil {
		return fmt.Errorf("invalid block: %w", err)
	}

	// omit checking existence of parent if block at lowest non-pruned view number
	if (block.View() == r.forest.LowestLevel) || (block.QC().View < r.forest.LowestLevel) {
		return nil
	}
	// for block whose parents are _not_ below the pruning height, we expect the parent to be known.
	if _, isParentKnown := r.forest.GetVertex(&block.QC().BlockID); !isParentKnown { // we are missing the parent
		return &types.ErrorMissingBlock{
			View:    block.QC().View,
			BlockID: block.QC().BlockID,
		}
	}
	return nil
}
