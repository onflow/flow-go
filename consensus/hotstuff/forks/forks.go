package forks

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/forest"
	"github.com/onflow/flow-go/module/mempool"
)

// ErrPrunedAncestry is a sentinel error: cannot resolve ancestry of block due to pruning
var ErrPrunedAncestry = errors.New("cannot resolve pruned ancestor")

// ancestryChain encapsulates a block, its parent (oneChain) and its grand-parent (twoChain).
// Given a chain structure like:
//
//	b <~ b' <~ b*
//
// where the QC certifying b is qc_b, this data structure looks like:
//
//	twoChain   oneChain     block
//	[b<-qc_b]  [b'<-qc_b']  [b*]
type ancestryChain struct {
	block    *BlockContainer
	oneChain *BlockQC
	twoChain *BlockQC
}

// Forks enforces structural validity of the consensus state and implements
// finalization rules as defined in Jolteon consensus https://arxiv.org/abs/2106.10362
// The same approach has later been adopted by the Diem team resulting in DiemBFT v4:
// https://developers.diem.com/papers/diem-consensus-state-machine-replication-in-the-diem-blockchain/2021-08-17.pdf
// Forks is NOT safe for concurrent use by multiple goroutines.
type Forks struct {
	notifier hotstuff.FinalizationConsumer
	forest   forest.LevelledForest

	finalizationCallback module.Finalizer
	newestView           uint64   // newestView is the highest view of block proposal stored in Forks
	lastFinalized        *BlockQC // lastFinalized is the QC that POINTS TO the most recently finalized locked block
}

var _ hotstuff.Forks = (*Forks)(nil)

func New(trustedRoot *BlockQC, finalizationCallback module.Finalizer, notifier hotstuff.FinalizationConsumer) (*Forks, error) {
	if (trustedRoot.Block.BlockID != trustedRoot.QC.BlockID) || (trustedRoot.Block.View != trustedRoot.QC.View) {
		return nil, model.NewConfigurationErrorf("invalid root: root qc is not pointing to root block")
	}

	forks := Forks{
		notifier:             notifier,
		finalizationCallback: finalizationCallback,
		forest:               *forest.NewLevelledForest(trustedRoot.Block.View),
		lastFinalized:        trustedRoot,
		newestView:           trustedRoot.Block.View,
	}

	// CAUTION: instead of a proposal, we use a normal block (without `SigData` and `LastViewTC`,
	// which would be possibly included in a full proposal). Per convention, we consider the
	// root block as already committed and enter a higher view.
	// Therefore, the root block's proposer signature and TC are irrelevant for consensus.
	trustedRootProposal := &model.Proposal{
		Block: trustedRoot.Block,
	}

	// verify and add root block to levelled forest
	err := forks.VerifyProposal(trustedRootProposal)
	if err != nil {
		return nil, fmt.Errorf("invalid root block: %w", err)
	}
	forks.forest.AddVertex(&BlockContainer{Proposal: trustedRootProposal})
	return &forks, nil
}

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

// AddProposal adds proposal to the consensus state. Performs verification to make sure that we don't
// add invalid proposals into consensus state.
// We assume that all blocks are fully verified. A valid block must satisfy all consistency
// requirements; otherwise we have a bug in the compliance layer.
// Expected errors during normal operations:
//   - model.ByzantineThresholdExceededError - new block results in conflicting finalized blocks
func (f *Forks) AddProposal(proposal *model.Proposal) error {
	err := f.VerifyProposal(proposal)
	if err != nil {
		if model.IsMissingBlockError(err) {
			return fmt.Errorf("cannot add proposal with missing parent: %s", err.Error())
		}
		// technically, this not strictly required. However, we leave this as a sanity check for now
		return fmt.Errorf("cannot add invalid proposal to Forks: %w", err)
	}
	err = f.UnverifiedAddProposal(proposal)
	if err != nil {
		return fmt.Errorf("error storing proposal in Forks: %w", err)
	}

	return nil
}

// IsKnownBlock checks whether block is known.
// UNVALIDATED: expects block to pass Forks.VerifyProposal(block)
func (f *Forks) IsKnownBlock(block *model.Block) bool {
	_, hasBlock := f.forest.GetVertex(block.BlockID)
	return hasBlock
}

// IsProcessingNeeded performs basic checks to determine whether block needs processing,
// only considering the block's height and hash.
// Returns false if any of the following conditions applies
//   - block view is _below_ the most recently finalized block
//   - the block already exists in the consensus state
//
// UNVALIDATED: expects block to pass Forks.VerifyProposal(block)
func (f *Forks) IsProcessingNeeded(block *model.Block) bool {
	if block.View < f.lastFinalized.Block.View || f.IsKnownBlock(block) {
		return false
	}
	return true
}

// UnverifiedAddProposal adds `proposal` to the consensus state and updates the
// latest finalized block, if possible.
// Calling this method with previously-processed blocks leaves the consensus state invariant
// (though, it will potentially cause some duplicate processing).
// UNVALIDATED: expects block to pass Forks.VerifyProposal(block)
// Error returns:
// * model.ByzantineThresholdExceededError if proposal's QC conflicts with an existing QC.
// * generic error in case of unexpected bug or internal state corruption
func (f *Forks) UnverifiedAddProposal(proposal *model.Proposal) error {
	if !f.IsProcessingNeeded(proposal.Block) {
		return nil
	}
	blockContainer := &BlockContainer{Proposal: proposal}
	block := blockContainer.Proposal.Block

	err := f.checkForConflictingQCs(block.QC)
	if err != nil {
		return err
	}
	f.checkForDoubleProposal(blockContainer)
	f.forest.AddVertex(blockContainer)
	if f.newestView < block.View {
		f.newestView = block.View
	}

	err = f.updateFinalizedBlockQC(blockContainer)
	if err != nil {
		return fmt.Errorf("updating consensus state failed: %w", err)
	}
	f.notifier.OnBlockIncorporated(block)
	return nil
}

// VerifyProposal checks a block for internal consistency and consistency with
// the current forest state. See forest.VerifyVertex for more detail.
// We assume that all blocks are fully verified. A valid block must satisfy all consistency
// requirements; otherwise we have a bug in the compliance layer.
// Error returns:
//   - model.MissingBlockError if the parent of the input proposal does not exist in the forest
//     (but is above the pruned view)
//   - generic error in case of unexpected bug or internal state corruption
func (f *Forks) VerifyProposal(proposal *model.Proposal) error {
	block := proposal.Block
	if block.View < f.forest.LowestLevel {
		return nil
	}
	blockContainer := &BlockContainer{Proposal: proposal}
	err := f.forest.VerifyVertex(blockContainer)
	if err != nil {
		if forest.IsInvalidVertexError(err) {
			return fmt.Errorf("cannot add proposal %x to forest: %s", block.BlockID, err.Error())
		}
		return fmt.Errorf("unexpected error verifying proposal vertex: %w", err)
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

// checkForConflictingQCs checks if qc conflicts with a stored Quorum Certificate.
// In case a conflicting QC is found, an ByzantineThresholdExceededError is returned.
//
// Two Quorum Certificates q1 and q2 are defined as conflicting iff:
//   - q1.View == q2.View
//   - q1.BlockID != q2.BlockID
//
// This means there are two Quorums for conflicting blocks at the same view.
// Per 'Observation 1' from the Jolteon paper https://arxiv.org/pdf/2106.10362v1.pdf, two
// conflicting QCs can exist if and only if the Byzantine threshold is exceeded.
// Error returns:
// * model.ByzantineThresholdExceededError if input QC conflicts with an existing QC.
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

// checkForDoubleProposal checks if the input proposal is a double proposal.
// A double proposal occurs when two proposals with the same view exist in Forks.
// If there is a double proposal, notifier.OnDoubleProposeDetected is triggered.
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

// updateFinalizedBlockQC updates the latest finalized block, if possible.
// This function should be called every time a new block is added to Forks.
// If the new block is the head of a 2-chain satisfying the finalization rule,
// then we update Forks.lastFinalizedBlockQC to the new latest finalized block.
// Calling this method with previously-processed blocks leaves the consensus state invariant.
// UNVALIDATED: assumes that relevant block properties are consistent with previous blocks
// Error returns:
//   - model.ByzantineThresholdExceededError if we are finalizing a block which is invalid to finalize.
//     This either indicates a critical internal bug / data corruption, or that the network Byzantine
//     threshold was exceeded, breaking the safety guarantees of HotStuff.
//   - generic error in case of unexpected bug or internal state corruption
func (f *Forks) updateFinalizedBlockQC(blockContainer *BlockContainer) error {
	ancestryChain, err := f.getTwoChain(blockContainer)
	if err != nil {
		// We expect that getTwoChain might error with a ErrPrunedAncestry. This error indicates that the
		// 2-chain of this block reaches _beyond_ the last finalized block. It is straight forward to show:
		// Lemma: Let B be a block whose 2-chain reaches beyond the last finalized block
		//        => B will not update the locked or finalized block
		if errors.Is(err, ErrPrunedAncestry) {
			// blockContainer's 2-chain reaches beyond the last finalized block
			// based on Lemma from above, we can skip attempting to update locked or finalized block
			return nil
		}
		if model.IsMissingBlockError(err) {
			// we are missing some un-pruned ancestry of blockContainer -> indicates corrupted internal state
			return fmt.Errorf("unexpected missing block while updating consensus state: %s", err.Error())
		}
		return fmt.Errorf("retrieving 2-chain ancestry failed: %w", err)
	}

	// Note: we assume that all stored blocks pass Forks.VerifyProposal(block);
	//       specifically, that Proposal's ViewNumber is strictly monotonously
	//       increasing which is enforced by LevelledForest.VerifyVertex(...)
	// We denote:
	//  * a DIRECT 1-chain as '<-'
	//  * a general 1-chain as '<~' (direct or indirect)
	// Jolteon's rule for finalizing block b is
	//     b <- b' <~ b*     (aka a DIRECT 1-chain PLUS any 1-chain)
	// where b* is the head block of the ancestryChain
	// Hence, we can finalize b as head of 2-chain, if and only the viewNumber of b' is exactly 1 higher than the view of b
	b := ancestryChain.twoChain
	if ancestryChain.oneChain.Block.View != b.Block.View+1 {
		return nil
	}
	return f.finalizeUpToBlock(b.QC)
}

// getTwoChain returns the 2-chain for the input block container b.
// See ancestryChain for documentation on the structure of the 2-chain.
// Returns ErrPrunedAncestry if any part of the 2-chain is below the last pruned view.
// Error returns:
//   - ErrPrunedAncestry if any part of the 2-chain is below the last pruned view.
//   - model.MissingBlockError if any block in the 2-chain does not exist in the forest
//     (but is above the pruned view)
//   - generic error in case of unexpected bug or internal state corruption
func (f *Forks) getTwoChain(blockContainer *BlockContainer) (*ancestryChain, error) {
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
	return &ancestryChain, nil
}

// getNextAncestryLevel retrieves parent from forest. Returns QCBlock for the parent,
// i.e. the parent block itself and the qc pointing to the parent, i.e. block.QC().
// UNVALIDATED: expects block to pass Forks.VerifyProposal(block)
// Error returns:
//   - ErrPrunedAncestry if the input block's parent is below the pruned view.
//   - model.MissingBlockError if the parent block does not exist in the forest
//     (but is above the pruned view)
//   - generic error in case of unexpected bug or internal state corruption
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
	parentBlock := parentVertex.(*BlockContainer).Proposal.Block
	// sanity check consistency between input block and parent
	if parentBlock.BlockID != block.QC.BlockID || parentBlock.View != block.QC.View {
		return nil, fmt.Errorf("parent/child mismatch while getting ancestry level: child: (id=%x, view=%d, qc.view=%d, qc.block_id=%x) parent: (id=%x, view=%d)",
			block.BlockID, block.View, block.QC.View, block.QC.BlockID, parentBlock.BlockID, parentBlock.View)
	}

	blockQC := BlockQC{Block: parentBlock, QC: block.QC}

	return &blockQC, nil
}

// finalizeUpToBlock finalizes all blocks up to (and including) the block pointed to by `qc`.
// Finalization starts with the child of `lastFinalizedBlockQC` (explicitly checked);
// and calls OnFinalizedBlock on the newly finalized blocks in increasing height order.
// Error returns:
//   - model.ByzantineThresholdExceededError if we are finalizing a block which is invalid to finalize.
//     This either indicates a critical internal bug / data corruption, or that the network Byzantine
//     threshold was exceeded, breaking the safety guarantees of HotStuff.
//   - generic error in case of bug or internal state corruption
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
				"finalizing blocks with view %d at conflicting forks: %x and %x",
				qc.View, qc.BlockID, f.lastFinalized.Block.BlockID,
			)}
		}
		return nil
	}
	// Have: qc.View > f.lastFinalizedBlockQC.View => finalizing new block

	// get Proposal and finalize everything up to the block's parent
	blockVertex, ok := f.forest.GetVertex(qc.BlockID) // require block to resolve parent
	if !ok {
		return fmt.Errorf("failed to get parent while finalizing blocks (qc.view=%d, qc.block_id=%x)", qc.View, qc.BlockID)
	}
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
		if mempool.IsBelowPrunedThresholdError(err) {
			// we should never see this error because we finalize blocks in strictly increasing view order
			return fmt.Errorf("unexpected error pruning forest, indicates corrupted state: %s", err.Error())
		}
		return fmt.Errorf("unexpected error while pruning forest: %w", err)
	}

	// notify other critical components about finalized block - all errors returned are considered critical
	err = f.finalizationCallback.MakeFinal(blockContainer.VertexID())
	if err != nil {
		return fmt.Errorf("finalization error in other component: %w", err)
	}

	// notify less important components about finalized block
	f.notifier.OnFinalizedBlock(block)
	return nil
}
