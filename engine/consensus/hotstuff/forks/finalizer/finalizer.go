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
	forest   forest.LevelledForest

	lastLocked *types.QCBlock // lastLockedBlockQC is the QC that POINTS TO the the most recently locked block

	lastFinalized *types.QCBlock // lastFinalizedBlockQC is the QC that POINTS TO the most recently finalized locked block
}

type ancestryChain struct {
	block           *BlockContainer
	oneChain      *types.QCBlock
	twoChain     *types.QCBlock
	threeChain    *types.QCBlock
}

func New(trustedRoot *types.QCBlock, notifier notifications.Distributor) (*Finalizer, error) {
	if (trustedRoot.BlockID() != trustedRoot.QC().BlockID) || (trustedRoot.View() != trustedRoot.QC().View) {
		return nil, &types.ErrorConfiguration{Msg: "invalid root: root qc is not pointing to root block"}
	}

	fnlzr := Finalizer{
		notifier:             notifier,
		forest:               *forest.NewLevelledForest(),
		lastLocked:      trustedRoot,
		lastFinalized:   trustedRoot,
	}

	// If rootBlock has view > 0, we can already pre-prune the levelled forest to the view below it.
	// Thereby, the levelled forest won't event store older (unnecessary) blocks
	if trustedRoot.View() > 0 {
		err := fnlzr.forest.PruneAtLevel(trustedRoot.View() - 1)
		if err != nil {
			return nil, fmt.Errorf("internal levelled forest error: %w", err)
		}
	}
	// verify and add root block to levelled forest
	err := fnlzr.VerifyBlock(trustedRoot.Block())
	if err != nil {
		return nil, fmt.Errorf("invalid root block: %w", err)
	}
	fnlzr.forest.AddVertex(&BlockContainer{block: trustedRoot.Block()})
	fnlzr.notifier.OnBlockIncorporated(trustedRoot.Block())
	return &fnlzr, nil
}

func (r *Finalizer) LockedBlock() *types.BlockProposal          { return r.lastLocked.Block() }
func (r *Finalizer) LockedBlockQC() *types.QuorumCertificate    { return r.lastLocked.QC() }
func (r *Finalizer) FinalizedBlock() *types.BlockProposal       { return r.lastFinalized.Block() }
func (r *Finalizer) FinalizedBlockQC() *types.QuorumCertificate { return r.lastFinalized.QC() }

// GetBlock returns block for given ID
func (r *Finalizer) GetBlock(blockID flow.Identifier) (*types.BlockProposal, bool) {
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
	_, hasBlock := r.forest.GetVertex(block.BlockID())
	return hasBlock
}

// isProcessingNeeded performs basic checks whether or not block needs processing
// only considering the block's height and Hash
// Returns false if any of the following conditions applies
//  * block view is _below_ the most recently finalized block
//  * known block
// UNVALIDATED: expects block to pass Finalizer.VerifyBlock(block)
func (r *Finalizer) IsProcessingNeeded(block *types.BlockProposal) bool {
	if block.View() < r.lastFinalized.View() || r.IsKnownBlock(block) {
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
	if qc.View > r.lastLocked.View() {
		return true
	}
	if (qc.View == r.lastLocked.View()) && (qc.BlockID == r.lastLocked.BlockID()) {
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
	if err := r.checkForByzantineQC(blockContainer.QC()); err != nil {
		return err
	}
	r.checkForDoubleProposal(blockContainer)
	r.forest.AddVertex(blockContainer)
	err := r.updateConsensusState(blockContainer)
	if err != nil {
		return fmt.Errorf("updating consensus state failed: %w", err)
	}
	r.notifier.OnBlockIncorporated(blockContainer.Block())
	return nil
}

// checkForByzantineQC checks if qc conflicts with a stored Quorum Certificate.
// In case a conflicting QC is found, an ErrorByzantineThresholdExceeded is returned.
//
// Two Quorum Certificates q1 and q2 are defined as conflicting iff:
//     * q1.View == q2.View
//     * q1.BlockID != q2.BlockID
// This means there are two Quorums for conflicting blocks at the same view.
// Per Lemma 1 from the HotStuff paper https://arxiv.org/abs/1803.05069v6, two
// conflicting QCs can exists if and onluy of the Byzantine threshold is exceeded.
func (r *Finalizer) checkForByzantineQC(qc *types.QuorumCertificate) error {
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
				conflictingQC := otherChild.(*BlockContainer).QC()
				return &hotstuff.ErrorByzantineThresholdExceeded{Evidence: fmt.Sprintf(
					"conflicting QCs at view %d: %v and %v",
					qc.View, qc.BlockID, conflictingQC.BlockID,
				)}
			}
		}
	}
	return nil
}

// checkForDoubleProposal checks if Block is a double proposal. In case it is,
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
		case *ErrorPrunedAncestry:
			return nil // for finalization, we ignore all blocks which do not have a full un-pruned 3-chain
		default:
			return fmt.Errorf("retrieving 3-chain ancestry failed: %w", err)
		}
	}
	r.updateLockedQc(ancestryChain)
	err = r.updateFinalizedBlockQc(ancestryChain)
	if err != nil {
		return fmt.Errorf("updating finazlied block failed: %w", err)
	}
	return nil
}

// getThreeChain returns the three chain or a ErrorPruned3Chain sentinel error
// to indicate that the 3-chain from blockContainer is (partially) pruned
func (r *Finalizer) getThreeChain(blockContainer *BlockContainer) (*ancestryChain, error) {
	ancestryChain := ancestryChain{block: blockContainer}

	var err error
	ancestryChain.oneChain, err = r.getNextAncestryLevel(blockContainer.Block())
	if err != nil {
		return nil, err
	}
	ancestryChain.twoChain, err = r.getNextAncestryLevel(ancestryChain.oneChain.Block())
	if err != nil {
		return nil, err
	}
	ancestryChain.threeChain, err = r.getNextAncestryLevel(ancestryChain.twoChain.Block())
	if err != nil {
		return nil, err
	}
	return &ancestryChain, nil
}

// getNextAncestryLevel parent from forest. Returns QCBlock for the parent,
// i.e. the parent block itself and the qc pointing to the parent, i.e. block.QC().
// UNVALIDATED: expects block to pass Finalizer.VerifyBlock(block)
func (r *Finalizer) getNextAncestryLevel(block *types.BlockProposal) (*types.QCBlock, error) {
	if block.QC().View < r.lastFinalized.View() {
		return nil, &ErrorPrunedAncestry{block: block}
	}
	parentVertex, parentBlockKnown := r.forest.GetVertex(block.QC().BlockID)
	if !parentBlockKnown {
		return nil, &types.ErrorMissingBlock{View: block.QC().View, BlockID: block.QC().BlockID}
	}
	qcBlock, err := types.NewQcBlock(block.QC(), parentVertex.(*BlockContainer).Block())
	if err != nil {
		return nil, err
	}
	return qcBlock, nil
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
	if ancestryChain.twoChain.View() <= r.lastLocked.View() {
		return
	}
	// update qc to newer block with any 2-chain on top of it:
	r.lastLocked = ancestryChain.twoChain
	r.lastFinalized = ancestryChain.twoChain
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
	if ancestryChain.oneChain.View() != b.View()+2 {
		return nil
	}
	parentQc := b.Block().QC()
	return r.finalizeUpToBlock(parentQc)
}

// finalizeUpToBlock finalizes all blocks up to (and including) the block pointed to by `blockQC`.
// Finalization starts with the child of `lastFinalizedBlockQC` (explicitly checked);
// and calls OnFinalizedBlock on the newly finalized blocks in the respective order
func (r *Finalizer) finalizeUpToBlock(blockQC *types.QuorumCertificate) error {
	if blockQC.View <= r.lastFinalized.View() {
		// Sanity check: the previously last Finalized Block must be an ancestor of `block`
		if r.lastFinalized.BlockID() != blockQC.BlockID {
			return &hotstuff.ErrorByzantineThresholdExceeded{Evidence: fmt.Sprintf(
				"finalizing blocks at conflicting forks: %v and %v",
				blockQC.BlockID, r.lastFinalized.BlockID(),
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
	blockVertex, _ := r.forest.GetVertex(blockQC.BlockID) // require block to resolve parent
	blockContainer := blockVertex.(*BlockContainer)
	err := r.finalizeUpToBlock(blockContainer.QC()) // finalize Parent, i.e. the block pointed to by the block's QC
	if err != nil {
		return err
	}

	// finalize block itself:
	r.lastFinalized, err = types.NewQcBlock(blockQC, blockContainer.Block())
	if err != nil {
		return fmt.Errorf("creation of QCBlock for new finalized block failed: %w", err)
	}
	err = r.forest.PruneAtLevel(blockContainer.View() - 1) // cannot underflow as of (2) ... unless we have a bug
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
	if _, isParentKnown := r.forest.GetVertex(block.QC().BlockID); !isParentKnown { // we are missing the parent
		return &types.ErrorMissingBlock{
			View:    block.QC().View,
			BlockID: block.QC().BlockID,
		}
	}
	return nil
}
