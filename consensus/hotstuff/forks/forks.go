package forks

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/forest"
)

// Forks implements HotStuff finalization logic
type Forks struct {
	notifier hotstuff.FinalizationConsumer
	forest   forest.LevelledForest

	finalizationCallback module.Finalizer
	newestView           uint64
	lastLocked           *BlockQC // lastLockedBlockQC is the QC that POINTS TO the the most recently locked block
	lastFinalized        *BlockQC // lastFinalizedBlockQC is the QC that POINTS TO the most recently finalized locked block
}

var _ hotstuff.Forks = (*Forks)(nil)

type ancestryChain struct {
	block      *BlockContainer
	oneChain   *BlockQC
	twoChain   *BlockQC
	threeChain *BlockQC
}

// ErrPrunedAncestry is a sentinel error: cannot resolve ancestry of block due to pruning
var ErrPrunedAncestry = errors.New("cannot resolve pruned ancestor")

func New(trustedRoot *BlockQC, finalizationCallback module.Finalizer, notifier hotstuff.FinalizationConsumer) (*Forks, error) {
	if (trustedRoot.Block.BlockID != trustedRoot.QC.BlockID) || (trustedRoot.Block.View != trustedRoot.QC.View) {
		return nil, model.NewConfigurationErrorf("invalid root: root qc is not pointing to root block")
	}

	fnlzr := Forks{
		notifier:             notifier,
		finalizationCallback: finalizationCallback,
		forest:               *forest.NewLevelledForest(trustedRoot.Block.View),
		lastLocked:           trustedRoot,
		lastFinalized:        trustedRoot,
		newestView:           trustedRoot.Block.View,
	}

	// CAUTION: instead of a proposal, we use a normal block (without `SigData` and `LastViewTC`, which would be
	// possibly included in a full proposal). Per convention, we consider the root block as already and enter a
	// higher view. Therefore, the root block's proposer signature and TC are irrelevant for consensus.
	trustedRootProposal := &model.Proposal{
		Block: trustedRoot.Block,
	}

	// verify and add root block to levelled forest
	err := fnlzr.VerifyProposal(trustedRootProposal)
	if err != nil {
		return nil, fmt.Errorf("invalid root block: %w", err)
	}
	fnlzr.forest.AddVertex(&BlockContainer{Proposal: trustedRootProposal})
	fnlzr.notifier.OnBlockIncorporated(trustedRoot.Block)
	return &fnlzr, nil
}

func (f *Forks) LockedBlock() *model.Block    { return f.lastLocked.Block }
func (f *Forks) FinalizedBlock() *model.Block { return f.lastFinalized.Block }
func (f *Forks) FinalizedView() uint64        { return f.lastFinalized.Block.View }
func (f *Forks) NewestView() uint64           { return f.newestView }

// GetProposal returns block for given ID
func (f *Forks) GetProposal(blockID flow.Identifier) (*model.Proposal, bool) {
	blockContainer, hasBlock := f.forest.GetVertex(blockID)
	if !hasBlock {
		return nil, false
	}
	return blockContainer.(*BlockContainer).Proposal, true
}

// GetProposalsForView returns all known proposals for the given view
func (f *Forks) GetProposalsForView(view uint64) []*model.Proposal {
	vertexIterator := f.forest.GetVerticesAtLevel(view)
	l := make([]*model.Proposal, 0, 1) // in the vast majority of cases, there will only be one proposal for a particular view
	for vertexIterator.HasNext() {
		v := vertexIterator.NextVertex().(*BlockContainer)
		l = append(l, v.Proposal)
	}
	return l
}

// IsKnownBlock checks whether block is known.
// UNVALIDATED: expects block to pass Forks.VerifyBlock(block)
func (f *Forks) IsKnownBlock(block *model.Block) bool {
	_, hasBlock := f.forest.GetVertex(block.BlockID)
	return hasBlock
}

// IsProcessingNeeded performs basic checks whether or not block needs processing
// only considering the block's height and hash
// Returns false if any of the following conditions applies
//  * block view is _below_ the most recently finalized block
//  * known block
// UNVALIDATED: expects block to pass Forks.VerifyBlock(block)
func (f *Forks) IsProcessingNeeded(block *model.Block) bool {
	if block.View < f.lastFinalized.Block.View || f.IsKnownBlock(block) {
		return false
	}
	return true
}

// IsSafeBlock returns true if block is safe to vote for
// (according to the definition in https://arxiv.org/abs/1803.05069v6).
// NO MODIFICATION of consensus state (read only)
// UNVALIDATED: expects block to pass Forks.VerifyBlock(block)
func (f *Forks) IsSafeBlock(block *model.Block) bool {
	// According to the paper, a block is considered a safe block if
	//  * it extends from locked block (safety rule),
	//  * or the view of the parent block is higher than the view number of locked block (liveness rule).
	// The two rules can be boiled down to the following:
	// 1. If block.QC.View is higher than locked view, it definitely is a safe block.
	// 2. If block.QC.View is lower than locked view, it definitely is not a safe block.
	// 3. If block.QC.View equals to locked view: parent must be the locked block.
	qc := block.QC
	if qc.View > f.lastLocked.Block.View {
		return true
	}
	if (qc.View == f.lastLocked.Block.View) && (qc.BlockID == f.lastLocked.Block.BlockID) {
		return true
	}
	return false
}

// UnverifiedAddProposal adds `proposal` to the consensus state.
// Calling this method with previously-processed blocks leaves the consensus state invariant
// (though, it will potentially cause some duplicate processing).
// UNVALIDATED: expects block to pass Forks.VerifyBlock(block)
func (f *Forks) UnverifiedAddProposal(proposal *model.Proposal) error {
	if !f.IsProcessingNeeded(proposal.Block) {
		return nil
	}
	blockContainer := &BlockContainer{Proposal: proposal}
	block := blockContainer.Proposal.Block
	if err := f.checkForConflictingQCs(block.QC); err != nil {
		return err
	}
	f.checkForDoubleProposal(blockContainer)
	f.forest.AddVertex(blockContainer)
	if f.newestView < block.View {
		f.newestView = block.View
	}
	err := f.updateConsensusState(blockContainer)
	if err != nil {
		return fmt.Errorf("updating consensus state failed: %w", err)
	}
	err = f.finalizationCallback.MakeValid(block.BlockID)
	if err != nil {
		return fmt.Errorf("MakeValid fails in other component: %w", err)
	}
	f.notifier.OnBlockIncorporated(block)
	return nil
}

// AddProposal adds proposal to the consensus state. Performs verification to make sure that we don't
// add invalid proposals into consensus state.
// Expected errors during normal operations:
//  * model.ByzantineThresholdExceededError - new block results in conflicting finalized blocks
func (f *Forks) AddProposal(proposal *model.Proposal) error {
	if err := f.VerifyProposal(proposal); err != nil {
		// technically, this not strictly required. However, we leave this as a sanity check for now
		return fmt.Errorf("cannot add invalid proposal to Forks: %w", err)
	}
	err := f.UnverifiedAddProposal(proposal)
	if err != nil {
		return fmt.Errorf("error storing proposal in Forks: %w", err)
	}

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
func (f *Forks) checkForConflictingQCs(qc *flow.QuorumCertificate) error {
	it := f.forest.GetVerticesAtLevel(qc.View)
	for it.HasNext() {
		otherBlock := it.NextVertex() // by construction, must have same view as qc.View
		if qc.BlockID != otherBlock.VertexID() {
			// * we have just found another block at the same view number as qc.View but with different hash
			// * if this block has a child c, this child will have
			//   c.qc.view = parentView
			//   c.qc.ID != parentBlockID
			// => conflicting qc
			otherChildren := f.forest.GetChildren(otherBlock.VertexID())
			if otherChildren.HasNext() {
				otherChild := otherChildren.NextVertex()
				conflictingQC := otherChild.(*BlockContainer).Proposal.Block.QC
				return model.ByzantineThresholdExceededError{Evidence: fmt.Sprintf(
					"conflicting QCs at view %d: %v and %v",
					qc.View, qc.BlockID, conflictingQC.BlockID,
				)}
			}
		}
	}
	return nil
}

// checkForDoubleProposal checks if Proposal is a double proposal. In case it is,
// notifier.OnDoubleProposeDetected is triggered
func (f *Forks) checkForDoubleProposal(container *BlockContainer) {
	block := container.Proposal.Block
	it := f.forest.GetVerticesAtLevel(block.View)
	for it.HasNext() {
		otherVertex := it.NextVertex() // by construction, must have same view as parentView
		if container.VertexID() != otherVertex.VertexID() {
			f.notifier.OnDoubleProposeDetected(block, otherVertex.(*BlockContainer).Proposal.Block)
		}
	}
}

// updateConsensusState updates consensus state.
// Calling this method with previously-processed blocks leaves the consensus state invariant.
// UNVALIDATED: assumes that relevant block properties are consistent with previous blocks
func (f *Forks) updateConsensusState(blockContainer *BlockContainer) error {
	ancestryChain, err := f.getThreeChain(blockContainer)
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

	f.updateLockedQc(ancestryChain)
	err = f.updateFinalizedBlockQc(ancestryChain)
	if err != nil {
		return fmt.Errorf("updating finalized block failed: %w", err)
	}
	return nil
}

// getThreeChain returns the three chain or a ErrorPrunedAncestry sentinel error
// to indicate that the 3-chain from blockContainer is (partially) pruned
func (f *Forks) getThreeChain(blockContainer *BlockContainer) (*ancestryChain, error) {
	ancestryChain := ancestryChain{block: blockContainer}

	var err error
	ancestryChain.oneChain, err = f.getNextAncestryLevel(blockContainer.Proposal.Block)
	if err != nil {
		return nil, err
	}
	ancestryChain.twoChain, err = f.getNextAncestryLevel(ancestryChain.oneChain.Block)
	if err != nil {
		return nil, err
	}
	ancestryChain.threeChain, err = f.getNextAncestryLevel(ancestryChain.twoChain.Block)
	if err != nil {
		return nil, err
	}
	return &ancestryChain, nil
}

// getNextAncestryLevel parent from forest. Returns QCBlock for the parent,
// i.e. the parent block itself and the qc pointing to the parent, i.e. block.QC().
// If the block's parent is below the pruned view, it will error with an ErrorPrunedAncestry.
// UNVALIDATED: expects block to pass Forks.VerifyBlock(block)
func (f *Forks) getNextAncestryLevel(block *model.Block) (*BlockQC, error) {
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
	if (block.View <= f.lastFinalized.Block.View) || (block.QC.View < f.lastFinalized.Block.View) {
		return nil, ErrPrunedAncestry
	}

	parentVertex, parentBlockKnown := f.forest.GetVertex(block.QC.BlockID)
	if !parentBlockKnown {
		return nil, model.MissingBlockError{View: block.QC.View, BlockID: block.QC.BlockID}
	}
	newBlock := parentVertex.(*BlockContainer).Proposal.Block
	if newBlock.BlockID != block.QC.BlockID || newBlock.View != block.QC.View {
		return nil, fmt.Errorf("mismatch between finalized block and QC")
	}

	blockQC := BlockQC{Block: newBlock, QC: block.QC}

	return &blockQC, nil
}

// updateLockedBlock updates `lastLockedBlockQC`
// We use the locking rule from 'Event-driven HotStuff Protocol' where the condition is:
//    * Consider the set S of all blocks that have a INDIRECT 2-chain on top of it
//    * The 'Locked Proposal' is the block in S with the _highest view number_ (newest);
// Calling this method with previously-processed blocks leaves consensus state invariant.
func (f *Forks) updateLockedQc(ancestryChain *ancestryChain) {
	if ancestryChain.twoChain.Block.View <= f.lastLocked.Block.View {
		return
	}
	// update qc to newer block with any 2-chain on top of it:
	f.lastLocked = ancestryChain.twoChain
}

// updateFinalizedBlockQc updates `lastFinalizedBlockQC`
// We use the finalization rule from 'Event-driven HotStuff Protocol' where the condition is:
//    * Consider the set S of all blocks that have a DIRECT 2-chain on top of it PLUS any 1-chain
//    * The 'Last finalized Proposal' is the block in S with the _highest view number_ (newest);
// Calling this method with previously-processed blocks leaves consensus state invariant.
func (f *Forks) updateFinalizedBlockQc(ancestryChain *ancestryChain) error {
	// Note: we assume that all stored blocks pass Forks.VerifyProposal(block);
	//       specifically, that Proposal's ViewNumber is strictly monotonously
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
	return f.finalizeUpToBlock(b.QC)
}

// finalizeUpToBlock finalizes all blocks up to (and including) the block pointed to by `blockQC`.
// Finalization starts with the child of `lastFinalizedBlockQC` (explicitly checked);
// and calls OnFinalizedBlock on the newly finalized blocks in the respective order
func (f *Forks) finalizeUpToBlock(qc *flow.QuorumCertificate) error {
	if qc.View < f.lastFinalized.Block.View {
		return model.ByzantineThresholdExceededError{Evidence: fmt.Sprintf(
			"finalizing blocks with view %d which is lower than previously finalized block at view %d",
			qc.View, f.lastFinalized.Block.View,
		)}
	}
	if qc.View == f.lastFinalized.Block.View {
		// Sanity check: the previously last Finalized Proposal must be an ancestor of `block`
		if f.lastFinalized.Block.BlockID != qc.BlockID {
			return model.ByzantineThresholdExceededError{Evidence: fmt.Sprintf(
				"finalizing blocks at conflicting forks: %v and %v",
				qc.BlockID, f.lastFinalized.Block.BlockID,
			)}
		}
		return nil
	}
	// Have: qc.View > f.lastFinalizedBlockQC.View => finalizing new block

	// get Proposal and finalize everything up to the block's parent
	blockVertex, _ := f.forest.GetVertex(qc.BlockID) // require block to resolve parent
	blockContainer := blockVertex.(*BlockContainer)
	block := blockContainer.Proposal.Block
	err := f.finalizeUpToBlock(block.QC) // finalize Parent, i.e. the block pointed to by the block's QC
	if err != nil {
		return err
	}

	if block.BlockID != qc.BlockID || block.View != qc.View {
		return fmt.Errorf("mismatch between finalized block and QC")
	}

	// finalize block itself:
	f.lastFinalized = &BlockQC{Block: block, QC: qc}
	err = f.forest.PruneUpToLevel(block.View)
	if err != nil {
		return fmt.Errorf("pruning levelled forest failed: %w", err)
	}

	// notify other critical components about finalized block
	err = f.finalizationCallback.MakeFinal(blockContainer.VertexID())
	if err != nil {
		return fmt.Errorf("finalization error in other component: %w", err)
	}

	// notify less important components about finalized block
	f.notifier.OnFinalizedBlock(block)
	return nil
}

// VerifyProposal checks block for validity
func (f *Forks) VerifyProposal(proposal *model.Proposal) error {
	block := proposal.Block
	if block.View < f.forest.LowestLevel {
		return nil
	}
	blockContainer := &BlockContainer{Proposal: proposal}
	err := f.forest.VerifyVertex(blockContainer)
	if err != nil {
		return fmt.Errorf("invalid block: %w", err)
	}

	// omit checking existence of parent if block at lowest non-pruned view number
	if (block.View == f.forest.LowestLevel) || (block.QC.View < f.forest.LowestLevel) {
		return nil
	}
	// for block whose parents are _not_ below the pruning height, we expect the parent to be known.
	if _, isParentKnown := f.forest.GetVertex(block.QC.BlockID); !isParentKnown { // we are missing the parent
		return model.MissingBlockError{
			View:    block.QC.View,
			BlockID: block.QC.BlockID,
		}
	}
	return nil
}
