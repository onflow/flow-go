package forks

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/forest"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool"
)

type ancestryChain2 struct {
	block    *BlockContainer2
	oneChain *model.CertifiedBlock
	twoChain *model.CertifiedBlock
}

// FinalityProof represents a finality proof for a block B. Finality in Jolteon/HotStuff is
// determined by the 2-chain rule:
//
//	There exists a _certified_ block C, such that B.View + 1 = C.View
type FinalityProof struct {
	Block          *model.Block
	CertifiedChild model.CertifiedBlock
}

// Forks enforces structural validity of the consensus state and implements
// finalization rules as defined in Jolteon consensus https://arxiv.org/abs/2106.10362
// The same approach has later been adopted by the Diem team resulting in DiemBFT v4:
// https://developers.diem.com/papers/diem-consensus-state-machine-replication-in-the-diem-blockchain/2021-08-17.pdf
// Forks is NOT safe for concurrent use by multiple goroutines.
type Forks2 struct {
	notifier hotstuff.FinalizationConsumer
	forest   forest.LevelledForest

	trustedRoot          *model.CertifiedBlock
	newestView           uint64 // newestView is the highest view of block proposal stored in Forks
	finalizationCallback module.Finalizer

	// lastFinalized holds the latest finalized block including the certified child as proof of finality.
	// CAUTION: is nil, when Forks has not yet finalized any blocks beyond the finalized root block it was initialized with
	lastFinalized *FinalityProof //
}

var _ hotstuff.Forks = (*Forks2)(nil)

func NewForks2(trustedRoot *model.CertifiedBlock, finalizationCallback module.Finalizer, notifier hotstuff.FinalizationConsumer) (*Forks2, error) {
	if (trustedRoot.Block.BlockID != trustedRoot.QC.BlockID) || (trustedRoot.Block.View != trustedRoot.QC.View) {
		return nil, model.NewConfigurationErrorf("invalid root: root QC is not pointing to root block")
	}

	forks := Forks2{
		notifier:             notifier,
		finalizationCallback: finalizationCallback,
		forest:               *forest.NewLevelledForest(trustedRoot.Block.View),
		newestView:           trustedRoot.Block.View,
		trustedRoot:          trustedRoot,
		lastFinalized:        nil,
	}

	// verify and add root block to levelled forest
	err := forks.EnsureBlockIsValidExtension(trustedRoot.Block)
	if err != nil {
		return nil, fmt.Errorf("invalid root block: %w", err)
	}
	forks.forest.AddVertex((*BlockContainer2)(trustedRoot.Block))
	return &forks, nil
}

// FinalizedView returns the largest view number that has been finalized so far
func (f *Forks2) FinalizedView() uint64 {
	if f.lastFinalized == nil {
		return f.trustedRoot.Block.View
	}
	return f.lastFinalized.Block.View
}

// FinalizedBlock returns the finalized block with the largest view number
func (f *Forks2) FinalizedBlock() *model.Block {
	if f.lastFinalized == nil {
		return f.trustedRoot.Block
	}
	return f.lastFinalized.Block
}

// FinalityProof returns the latest finalized block and a certified child from
// the subsequent view, which proves finality.
// CAUTION: method returns (nil, false), when Forks has not yet finalized any
// blocks beyond the finalized root block it was initialized with.
func (f *Forks2) FinalityProof() (*FinalityProof, bool) {
	return f.lastFinalized, f.lastFinalized == nil
}

// NewestView returns the largest view number of all proposals that were added to Forks.
func (f *Forks2) NewestView() uint64 { return f.newestView }

// GetBlock returns block for given ID
func (f *Forks2) GetBlock(blockID flow.Identifier) (*model.Block, bool) {
	blockContainer, hasBlock := f.forest.GetVertex(blockID)
	if !hasBlock {
		return nil, false
	}
	return blockContainer.(*BlockContainer2).Block(), true
}

// GetProposalsForView returns all known proposals for the given view
func (f *Forks2) GetProposalsForView(view uint64) []*model.Block {
	vertexIterator := f.forest.GetVerticesAtLevel(view)
	l := make([]*model.Block, 0, 1) // in the vast majority of cases, there will only be one proposal for a particular view
	for vertexIterator.HasNext() {
		v := vertexIterator.NextVertex()
		l = append(l, v.(*BlockContainer2).Block())
	}
	return l
}

func (f *Forks2) AddCertifiedBlock(block *model.CertifiedBlock) error {
	err := f.VerifyProposal(block.Block)
	if err != nil {
		if model.IsMissingBlockError(err) {
			return fmt.Errorf("cannot add proposal with missing parent: %s", err.Error())
		}
		// technically, this not strictly required. However, we leave this as a sanity check for now
		return fmt.Errorf("cannot add invalid proposal to Forks: %w", err)
	}
}

// AddProposal adds proposal to the consensus state. Performs verification to make sure that we don't
// add invalid proposals into consensus state.
// We assume that all blocks are fully verified. A valid block must satisfy all consistency
// requirements; otherwise we have a bug in the compliance layer.
// Expected errors during normal operations:
//   - model.ByzantineThresholdExceededError - new block results in conflicting finalized blocks
func (f *Forks2) AddProposal(proposal *model.Block) error {
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
func (f *Forks2) IsKnownBlock(block *model.Block) bool {
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
func (f *Forks2) IsProcessingNeeded(block *model.Block) bool {
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
func (f *Forks2) UnverifiedAddProposal(proposal *model.Block) error {
	if !f.IsProcessingNeeded(proposal.Block) {
		return nil
	}
	blockContainer := &BlockContainer2{Proposal: proposal}
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

// EnsureBlockIsValidExtension checks that the given block is a valid extension to the tree
// of blocks already stored. Specifically, the following condition are enforced, which
// are critical to the correctness of Forks:
//
//  1. If block with the same ID is already stored, their views must be identical.
//
//  2. The block's view must be strictly larger than the view of its parent
//
//  3. The parent must already be stored (or below the pruning height)
//
// Exclusions to these rules (by design):
// Let W denote the view of block's parent (i.e. W := block.QC.View) and F the latest
// finalized view.
//
//	  (i) If block.View < F, adding the block would be a no-op. Such blocks are considered
//	      compatible (principle of vacuous truth), i.e. we skip checking 1, 2, 3.
//	 (ii) If block.View == F, we do not inspect the QC / parent at all (skip 2 and 3).
//	      This exception is important for compatability with genesis or spork-root blocks,
//	      which not contain a QCs.
//	(iii) If block.View > F, but block.QC.View < F the parent has already been pruned. In
//	      this case, we omit rule 3. (principle of vacuous truth applied to the parent)
//
// We assume that all blocks are fully verified. A valid block must satisfy all consistency
// requirements; otherwise we have a bug in the compliance layer.
//
// Error returns:
//   - model.MissingBlockError if the parent of the input proposal does not exist in the forest
//     (but is above the pruned view). Represents violation of condition 3. From the perspective
//     of Forks, this error is benign.
//   - Violation of condition 1. or 2. results in an exception. This error is a critical failure,
//     as Forks generally cannot handle invalid blocks, as they could lead to hidden state corruption.
//   - generic error in case of unexpected bug or internal state corruption
func (f *Forks2) EnsureBlockIsValidExtension(block *model.Block) error {
	if block.View < f.forest.LowestLevel { // exclusion (i)
		return nil
	}

	// LevelledForest enforces conditions 1. and 2. including the respective exclusions (ii) and (iii).
	blockContainer := (*BlockContainer2)(block)
	err := f.forest.VerifyVertex(blockContainer)
	if err != nil {
		if forest.IsInvalidVertexError(err) {
			return fmt.Errorf("not a valid vertex for block tree: %w", irrecoverable.NewException(err))
		}
		return fmt.Errorf("block tree generated unexpected error validating vertex: %w", err)
	}

	// Condition 3:
	// LevelledForest implements a more generalized algorithm that also works for disjoint graphs.
	// Therefore, LevelledForest _not_ enforce condition 3. Here, we additionally require that the
	// pending blocks form a tree (connected graph), i.e. we need to enforce condition 3
	if (block.View == f.forest.LowestLevel) || (block.QC.View < f.forest.LowestLevel) { // exclusion (ii) and (iii)
		return nil
	}
	// for block whose parents are _not_ below the pruning height, we expect the parent to be known.
	if _, isParentKnown := f.forest.GetVertex(block.QC.BlockID); !isParentKnown { // missing parent
		return model.MissingBlockError{
			View:    block.QC.View,
			BlockID: block.QC.BlockID,
		}
	}
	return nil
}

// checkForConflictingQCs checks if QC conflicts with a stored Quorum Certificate.
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
func (f *Forks2) checkForConflictingQCs(qc *flow.QuorumCertificate) error {
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
				conflictingQC := otherChild.(*BlockContainer2).Proposal.Block.QC
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
func (f *Forks2) checkForDoubleProposal(container *BlockContainer2) {
	block := container.Proposal.Block
	it := f.forest.GetVerticesAtLevel(block.View)
	for it.HasNext() {
		otherVertex := it.NextVertex() // by construction, must have same view as parentView
		if container.VertexID() != otherVertex.VertexID() {
			f.notifier.OnDoubleProposeDetected(block, otherVertex.(*BlockContainer2).Proposal.Block)
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
func (f *Forks2) updateFinalizedBlockQC(blockContainer *BlockContainer2) error {
	ancestryChain2, err := f.getTwoChain(blockContainer)
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
	b := ancestryChain2.twoChain
	if ancestryChain2.oneChain.Block.View != b.Block.View+1 {
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
func (f *Forks2) getTwoChain(blockContainer *BlockContainer2) (*ancestryChain2, error) {
	ancestryChain2 := ancestryChain2{block: blockContainer}

	var err error
	ancestryChain2.oneChain, err = f.getNextAncestryLevel(blockContainer.Proposal.Block)
	if err != nil {
		return nil, err
	}
	ancestryChain2.twoChain, err = f.getNextAncestryLevel(ancestryChain2.oneChain.Block)
	if err != nil {
		return nil, err
	}
	return &ancestryChain2, nil
}

// getNextAncestryLevel retrieves parent from forest. Returns QCBlock for the parent,
// i.e. the parent block itself and the qc pointing to the parent, i.e. block.QC().
// UNVALIDATED: expects block to pass Forks.VerifyProposal(block)
// Error returns:
//   - ErrPrunedAncestry if the input block's parent is below the pruned view.
//   - model.MissingBlockError if the parent block does not exist in the forest
//     (but is above the pruned view)
//   - generic error in case of unexpected bug or internal state corruption
func (f *Forks2) getNextAncestryLevel(block *model.Block) (*model.CertifiedBlock, error) {
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
	parentBlock := parentVertex.(*BlockContainer2).Proposal.Block
	// sanity check consistency between input block and parent
	if parentBlock.BlockID != block.QC.BlockID || parentBlock.View != block.QC.View {
		return nil, fmt.Errorf("parent/child mismatch while getting ancestry level: child: (id=%x, view=%d, qc.view=%d, qc.block_id=%x) parent: (id=%x, view=%d)",
			block.BlockID, block.View, block.QC.View, block.QC.BlockID, parentBlock.BlockID, parentBlock.View)
	}

	blockQC := model.CertifiedBlock{Block: parentBlock, QC: block.QC}

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
func (f *Forks2) finalizeUpToBlock(qc *flow.QuorumCertificate) error {
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
	blockContainer := blockVertex.(*BlockContainer2)
	block := blockContainer.Proposal.Block
	err := f.finalizeUpToBlock(block.QC) // finalize Parent, i.e. the block pointed to by the block's QC
	if err != nil {
		return err
	}

	if block.BlockID != qc.BlockID || block.View != qc.View {
		return fmt.Errorf("mismatch between finalized block and QC")
	}

	// finalize block itself:
	f.lastFinalized = &model.CertifiedBlock{Block: block, QC: qc}
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
