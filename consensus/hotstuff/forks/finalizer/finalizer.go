package finalizer

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/forks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/forest"
)

// Finalizer implements HotStuff finalization logic
type Finalizer struct {
	notifier hotstuff.FinalizationConsumer
	forest   forest.LevelledForest

	finalizationCallback module.Finalizer
	lastLocked           *forks.BlockQC // lastLockedBlockQC is the QC that POINTS TO the the most recently locked block
	lastFinalized        *forks.BlockQC // lastFinalizedBlockQC is the QC that POINTS TO the most recently finalized locked block
}

type ancestryChain struct {
	block      *BlockContainer
	oneChain   *forks.BlockQC
	twoChain   *forks.BlockQC
	threeChain *forks.BlockQC
}

// ErrPrunedAncestry is a sentinel error: cannot resolve ancestry of block due to pruning
var ErrPrunedAncestry = errors.New("cannot resolve pruned ancestor")

func New(trustedRoot *forks.BlockQC, finalizationCallback module.Finalizer, notifier hotstuff.FinalizationConsumer) (*Finalizer, error) {
	if (trustedRoot.Block.BlockID != trustedRoot.QC.BlockID) || (trustedRoot.Block.View != trustedRoot.QC.View) {
		return nil, &model.ConfigurationError{Msg: "invalid root: root qc is not pointing to root block"}
	}

	fnlzr := Finalizer{
		notifier:             notifier,
		finalizationCallback: finalizationCallback,
		forest:               *forest.NewLevelledForest(trustedRoot.Block.View),
		lastLocked:           trustedRoot,
		lastFinalized:        trustedRoot,
	}
	// verify and add root block to levelled forest
	err := fnlzr.VerifyBlock(trustedRoot.Block)
	if err != nil {
		return nil, fmt.Errorf("invalid root block: %w", err)
	}
	fnlzr.forest.AddVertex(&BlockContainer{Block: trustedRoot.Block})
	fnlzr.notifier.OnBlockIncorporated(trustedRoot.Block)
	return &fnlzr, nil
}

func (r *Finalizer) LockedBlock() *model.Block                 { return r.lastLocked.Block }
func (r *Finalizer) LockedBlockQC() *flow.QuorumCertificate    { return r.lastLocked.QC }
func (r *Finalizer) FinalizedBlock() *model.Block              { return r.lastFinalized.Block }
func (r *Finalizer) FinalizedView() uint64                     { return r.lastFinalized.Block.View }
func (r *Finalizer) FinalizedBlockQC() *flow.QuorumCertificate { return r.lastFinalized.QC }

// GetBlock returns block for given ID
func (r *Finalizer) GetBlock(blockID flow.Identifier) (*model.Block, bool) {
	blockContainer, hasBlock := r.forest.GetVertex(blockID)
	if !hasBlock {
		return nil, false
	}
	return blockContainer.(*BlockContainer).Block, true
}

// GetBlock returns all known blocks for the given
func (r *Finalizer) GetBlocksForView(view uint64) []*model.Block {
	vertexIterator := r.forest.GetVerticesAtLevel(view)
	l := make([]*model.Block, 0, 1) // in the vast majority of cases, there will only be one proposal for a particular view
	for vertexIterator.HasNext() {
		v := vertexIterator.NextVertex().(*BlockContainer)
		l = append(l, v.Block)
	}
	return l
}

// IsKnownBlock checks whether block is known.
// UNVALIDATED: expects block to pass Finalizer.VerifyBlock(block)
func (r *Finalizer) IsKnownBlock(block *model.Block) bool {
	_, hasBlock := r.forest.GetVertex(block.BlockID)
	return hasBlock
}

// IsProcessingNeeded performs basic checks whether or not block needs processing
// only considering the block's height and hash
// Returns false if any of the following conditions applies
//  * block view is _below_ the most recently finalized block
//  * known block
// UNVALIDATED: expects block to pass Finalizer.VerifyBlock(block)
func (r *Finalizer) IsProcessingNeeded(block *model.Block) bool {
	if block.View < r.lastFinalized.Block.View || r.IsKnownBlock(block) {
		return false
	}
	return true
}

// IsSafeBlock returns true if block is safe to vote for
// (according to the definition in https://arxiv.org/abs/1803.05069v6).
// NO MODIFICATION of consensus state (read only)
// UNVALIDATED: expects block to pass Finalizer.VerifyBlock(block)
func (r *Finalizer) IsSafeBlock(block *model.Block) bool {
	// According to the paper, a block is considered a safe block if
	//  * it extends from locked block (safety rule),
	//  * or the view of the parent block is higher than the view number of locked block (liveness rule).
	// The two rules can be boiled down to the following:
	// 1. If block.QC.View is higher than locked view, it definitely is a safe block.
	// 2. If block.QC.View is lower than locked view, it definitely is not a safe block.
	// 3. If block.QC.View equals to locked view: parent must be the locked block.
	qc := block.QC
	if qc.View > r.lastLocked.Block.View {
		return true
	}
	if (qc.View == r.lastLocked.Block.View) && (qc.BlockID == r.lastLocked.Block.BlockID) {
		return true
	}
	return false
}

// ProcessBlock adds `block` to the consensus state.
// Calling this method with previously-processed blocks leaves the consensus state invariant
// (though, it will potentially cause some duplicate processing).
// UNVALIDATED: expects block to pass Finalizer.VerifyBlock(block)
func (r *Finalizer) AddBlock(block *model.Block) error {
	if !r.IsProcessingNeeded(block) {
		return nil
	}
	blockContainer := &BlockContainer{Block: block}
	if err := r.checkForConflictingQCs(blockContainer.Block.QC); err != nil {
		return err
	}
	r.checkForDoubleProposal(blockContainer)
	r.forest.AddVertex(blockContainer)
	err := r.updateConsensusState(blockContainer)
	if err != nil {
		return fmt.Errorf("updating consensus state failed: %w", err)
	}
	err = r.finalizationCallback.MakeValid(blockContainer.Block.BlockID)
	if err != nil {
		return fmt.Errorf("MakeValid fails in other component: %w", err)
	}
	r.notifier.OnBlockIncorporated(blockContainer.Block)
	return nil
}

// checkForConflictingQCs checks if qc conflicts with a stored Quorum Certificate.
// In case a conflicting QC is found, an ByzantineThresholdExceededError is returned.
//
// Two Quorum Certificates q1 and q2 are defined as conflicting iff:
//     * q1.View == q2.View
//     * q1.BlockID != q2.BlockID
// This means there are two Quorums for conflicting blocks at the same view.
// Per Lemma 1 from the HotStuff paper https://arxiv.org/abs/1803.05069v6, two
// conflicting QCs can exists if and onluy of the Byzantine threshold is exceeded.
func (r *Finalizer) checkForConflictingQCs(qc *flow.QuorumCertificate) error {
	it := r.forest.GetVerticesAtLevel(qc.View)
	for it.HasNext() {
		otherBlock := it.NextVertex() // by construction, must have same view as qc.View
		if qc.BlockID != otherBlock.VertexID() {
			// * we have just found another block at the same view number as qc.View but with different hash
			// * if this block has a child c, this child will have
			//   c.qc.view = parentView
			//   c.qc.ID != parentBlockID
			// => conflicting qc
			otherChildren := r.forest.GetChildren(otherBlock.VertexID())
			if otherChildren.HasNext() {
				otherChild := otherChildren.NextVertex()
				conflictingQC := otherChild.(*BlockContainer).Block.QC
				return model.ByzantineThresholdExceededError{Evidence: fmt.Sprintf(
					"conflicting QCs at view %d: %v and %v",
					qc.View, qc.BlockID, conflictingQC.BlockID,
				)}
			}
		}
	}
	return nil
}

// checkForDoubleProposal checks if Block is a double proposal. In case it is,
// notifier.OnDoubleProposeDetected is triggered
func (r *Finalizer) checkForDoubleProposal(container *BlockContainer) {
	it := r.forest.GetVerticesAtLevel(container.Block.View)
	for it.HasNext() {
		otherVertex := it.NextVertex() // by construction, must have same view as parentView
		if container.VertexID() != otherVertex.VertexID() {
			r.notifier.OnDoubleProposeDetected(container.Block, otherVertex.(*BlockContainer).Block)
		}
	}
}

// updateConsensusState updates consensus state.
// Calling this method with previously-processed blocks leaves the consensus state invariant.
// UNVALIDATED: assumes that relevant block properties are consistent with previous blocks
func (r *Finalizer) updateConsensusState(blockContainer *BlockContainer) error {
	ancestryChain, err := r.getThreeChain(blockContainer)
	// We expect that getThreeChain might error with a ErrorPrunedAncestry. This error indicates that the
	// 3-chain of this block reaches _beyond_ the last finalized block. It is straight forward to show:
	// Lemma: Let B be a block whose 3-chain reaches beyond the last finalized block
	//        => B will not update the locked or finalized block
	if errors.Is(err, ErrPrunedAncestry) { // blockContainer's 3-chain reaches beyond the last finalized block
		// based on Lemma from above, we can skip attempting to update locked or finalized block
		return nil
	}
	if err != nil { // otherwise, there is an unknown error that we need to escalate to the higher-level application logic
		return fmt.Errorf("retrieving 3-chain ancestry failed: %w", err)
	}

	r.updateLockedQc(ancestryChain)
	err = r.updateFinalizedBlockQc(ancestryChain)
	if err != nil {
		return fmt.Errorf("updating finalized block failed: %w", err)
	}
	return nil
}

// getThreeChain returns the three chain or a ErrorPrunedAncestry sentinel error
// to indicate that the 3-chain from blockContainer is (partially) pruned
func (r *Finalizer) getThreeChain(blockContainer *BlockContainer) (*ancestryChain, error) {
	ancestryChain := ancestryChain{block: blockContainer}

	var err error
	ancestryChain.oneChain, err = r.getNextAncestryLevel(blockContainer.Block)
	if err != nil {
		return nil, err
	}
	ancestryChain.twoChain, err = r.getNextAncestryLevel(ancestryChain.oneChain.Block)
	if err != nil {
		return nil, err
	}
	ancestryChain.threeChain, err = r.getNextAncestryLevel(ancestryChain.twoChain.Block)
	if err != nil {
		return nil, err
	}
	return &ancestryChain, nil
}

// getNextAncestryLevel parent from forest. Returns QCBlock for the parent,
// i.e. the parent block itself and the qc pointing to the parent, i.e. block.QC().
// If the block's parent is below the pruned view, it will error with an ErrorPrunedAncestry.
// UNVALIDATED: expects block to pass Finalizer.VerifyBlock(block)
func (r *Finalizer) getNextAncestryLevel(block *model.Block) (*forks.BlockQC, error) {
	// The finalizer prunes all blocks in forest which are below the most recently finalized block.
	// Hence, we have a pruned ancestry if and only if either of the following conditions applies:
	//    (a) if a block's parent view (i.e. block.QC.View) is below the most recently finalized block.
	//    (b) if a block's view is equal to the most recently finalized block.
	// Caution:
	// * Under normal operation, case (b) is covered by the logic for case (a)
	// * However, the existence of a genesis block requires handling case (b) explicitly:
	//   The root block is specified and trusted by the node operator. If the root block is the
	//   genesis block, it might not contain a qc pointing to a parent (as there is no parent).
	//   In this case, condition (a) cannot be evaluated.
	if (block.View <= r.lastFinalized.Block.View) || (block.QC.View < r.lastFinalized.Block.View) {
		return nil, ErrPrunedAncestry
	}

	parentVertex, parentBlockKnown := r.forest.GetVertex(block.QC.BlockID)
	if !parentBlockKnown {
		return nil, model.MissingBlockError{View: block.QC.View, BlockID: block.QC.BlockID}
	}
	newBlock := parentVertex.(*BlockContainer).Block
	if newBlock.BlockID != block.QC.BlockID || newBlock.View != block.QC.View {
		return nil, fmt.Errorf("mismatch between finalized block and QC")
	}

	blockQC := forks.BlockQC{Block: newBlock, QC: block.QC}

	return &blockQC, nil
}

// updateLockedBlock updates `lastLockedBlockQC`
// We use the locking rule from 'Event-driven HotStuff Protocol' where the condition is:
//    * Consider the set S of all blocks that have a INDIRECT 2-chain on top of it
//    * The 'Locked Block' is the block in S with the _highest view number_ (newest);
// Calling this method with previously-processed blocks leaves consensus state invariant.
func (r *Finalizer) updateLockedQc(ancestryChain *ancestryChain) {
	if ancestryChain.twoChain.Block.View <= r.lastLocked.Block.View {
		return
	}
	// update qc to newer block with any 2-chain on top of it:
	r.lastLocked = ancestryChain.twoChain
}

// updateFinalizedBlockQc updates `lastFinalizedBlockQC`
// We use the finalization rule from 'Event-driven HotStuff Protocol' where the condition is:
//    * Consider the set S of all blocks that have a DIRECT 2-chain on top of it PLUS any 1-chain
//    * The 'Last finalized Block' is the block in S with the _highest view number_ (newest);
// Calling this method with previously-processed blocks leaves consensus state invariant.
func (r *Finalizer) updateFinalizedBlockQc(ancestryChain *ancestryChain) error {
	// Note: we assume that all stored blocks pass Finalizer.VerifyBlock(block);
	//       specifically, that Block's ViewNumber is strictly monotonously
	//       increasing which is enforced by LevelledForest.VerifyVertex(...)
	// We denote:
	//  * a DIRECT 1-chain as '<-'
	//  * a general 1-chain as '<~' (direct or indirect)
	// The rule from 'Event-driven HotStuff' for finalizing block b is
	//     b <- b' <- b'' <~ b*     (aka a DIRECT 2-chain PLUS any 1-chain)
	// where b* is the input block to this method.
	// Hence, we can finalize b, if and only the viewNumber of b'' is exactly 2 higher than the view of b
	b := ancestryChain.threeChain // note that b is actually not the block itself here but rather the QC pointing to it
	if ancestryChain.oneChain.Block.View != b.Block.View+2 {
		return nil
	}
	return r.finalizeUpToBlock(b.QC)
}

// finalizeUpToBlock finalizes all blocks up to (and including) the block pointed to by `blockQC`.
// Finalization starts with the child of `lastFinalizedBlockQC` (explicitly checked);
// and calls OnFinalizedBlock on the newly finalized blocks in the respective order
func (r *Finalizer) finalizeUpToBlock(qc *flow.QuorumCertificate) error {
	if qc.View < r.lastFinalized.Block.View {
		return model.ByzantineThresholdExceededError{Evidence: fmt.Sprintf(
			"finalizing blocks with view %d which is lower than previously finalized block at view %d",
			qc.View, r.lastFinalized.Block.View,
		)}
	}
	if qc.View == r.lastFinalized.Block.View {
		// Sanity check: the previously last Finalized Block must be an ancestor of `block`
		if r.lastFinalized.Block.BlockID != qc.BlockID {
			return model.ByzantineThresholdExceededError{Evidence: fmt.Sprintf(
				"finalizing blocks at conflicting forks: %v and %v",
				qc.BlockID, r.lastFinalized.Block.BlockID,
			)}
		}
		return nil
	}
	// Have: qc.View > r.lastFinalizedBlockQC.View => finalizing new block

	// get Block and finalize everything up to the block's parent
	blockVertex, _ := r.forest.GetVertex(qc.BlockID) // require block to resolve parent
	blockContainer := blockVertex.(*BlockContainer)
	err := r.finalizeUpToBlock(blockContainer.Block.QC) // finalize Parent, i.e. the block pointed to by the block's QC
	if err != nil {
		return err
	}

	block := blockContainer.Block
	if block.BlockID != qc.BlockID || block.View != qc.View {
		return fmt.Errorf("mismatch between finalized block and QC")
	}

	// finalize block itself:
	r.lastFinalized = &forks.BlockQC{Block: block, QC: qc}
	err = r.forest.PruneUpToLevel(blockContainer.Block.View)
	if err != nil {
		return fmt.Errorf("pruning levelled forest failed: %w", err)
	}

	// notify other critical components about finalized block
	err = r.finalizationCallback.MakeFinal(blockContainer.VertexID())
	if err != nil {
		return fmt.Errorf("finalization error in other component: %w", err)
	}

	// notify less important components about finalized block
	r.notifier.OnFinalizedBlock(blockContainer.Block)
	return nil
}

// VerifyBlock checks block for validity
func (r *Finalizer) VerifyBlock(block *model.Block) error {
	if block.View < r.forest.LowestLevel {
		return nil
	}
	blockContainer := &BlockContainer{Block: block}
	err := r.forest.VerifyVertex(blockContainer)
	if err != nil {
		return fmt.Errorf("invalid block: %w", err)
	}

	// omit checking existence of parent if block at lowest non-pruned view number
	if (block.View == r.forest.LowestLevel) || (block.QC.View < r.forest.LowestLevel) {
		return nil
	}
	// for block whose parents are _not_ below the pruning height, we expect the parent to be known.
	if _, isParentKnown := r.forest.GetVertex(block.QC.BlockID); !isParentKnown { // we are missing the parent
		return model.MissingBlockError{
			View:    block.QC.View,
			BlockID: block.QC.BlockID,
		}
	}
	return nil
}
