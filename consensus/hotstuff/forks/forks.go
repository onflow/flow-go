package forks

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/forest"
)

// Forks enforces structural validity of the consensus state and implements
// finalization rules as defined in Jolteon consensus https://arxiv.org/abs/2106.10362
// The same approach has later been adopted by the Diem team resulting in DiemBFT v4:
// https://developers.diem.com/papers/diem-consensus-state-machine-replication-in-the-diem-blockchain/2021-08-17.pdf
// Forks is NOT safe for concurrent use by multiple goroutines.
type Forks struct {
	finalizationCallback module.Finalizer
	notifier             hotstuff.FollowerConsumer
	forest               forest.LevelledForest
	trustedRoot          *model.CertifiedBlock

	// finalityProof holds the latest finalized block including the certified child as proof of finality.
	// CAUTION: is nil, when Forks has not yet finalized any blocks beyond the finalized root block it was initialized with
	finalityProof *hotstuff.FinalityProof
}

var _ hotstuff.Forks = (*Forks)(nil)

func New(trustedRoot *model.CertifiedBlock, finalizationCallback module.Finalizer, notifier hotstuff.FollowerConsumer) (*Forks, error) {
	if (trustedRoot.Block.BlockID != trustedRoot.CertifyingQC.BlockID) || (trustedRoot.Block.View != trustedRoot.CertifyingQC.View) {
		return nil, model.NewConfigurationErrorf("invalid root: root QC is not pointing to root block")
	}

	forks := Forks{
		finalizationCallback: finalizationCallback,
		notifier:             notifier,
		forest:               *forest.NewLevelledForest(trustedRoot.Block.View),
		trustedRoot:          trustedRoot,
		finalityProof:        nil,
	}

	// verify and add root block to levelled forest
	err := forks.EnsureBlockIsValidExtension(trustedRoot.Block)
	if err != nil {
		return nil, fmt.Errorf("invalid root block %v: %w", trustedRoot.ID(), err)
	}
	forks.forest.AddVertex(ToBlockContainer2(trustedRoot.Block))
	return &forks, nil
}

// FinalizedView returns the largest view number where a finalized block is known
func (f *Forks) FinalizedView() uint64 {
	if f.finalityProof == nil {
		return f.trustedRoot.Block.View
	}
	return f.finalityProof.Block.View
}

// FinalizedBlock returns the finalized block with the largest view number
func (f *Forks) FinalizedBlock() *model.Block {
	if f.finalityProof == nil {
		return f.trustedRoot.Block
	}
	return f.finalityProof.Block
}

// FinalityProof returns the latest finalized block and a certified child from
// the subsequent view, which proves finality.
// CAUTION: method returns (nil, false), when Forks has not yet finalized any
// blocks beyond the finalized root block it was initialized with.
func (f *Forks) FinalityProof() (*hotstuff.FinalityProof, bool) {
	return f.finalityProof, f.finalityProof != nil
}

// GetBlock returns (BlockProposal, true) if the block with the specified
// id was found and (nil, false) otherwise.
func (f *Forks) GetBlock(blockID flow.Identifier) (*model.Block, bool) {
	blockContainer, hasBlock := f.forest.GetVertex(blockID)
	if !hasBlock {
		return nil, false
	}
	return blockContainer.(*BlockContainer).Block(), true
}

// GetBlocksForView returns all known blocks for the given view
func (f *Forks) GetBlocksForView(view uint64) []*model.Block {
	vertexIterator := f.forest.GetVerticesAtLevel(view)
	blocks := make([]*model.Block, 0, 1) // in the vast majority of cases, there will only be one proposal for a particular view
	for vertexIterator.HasNext() {
		v := vertexIterator.NextVertex()
		blocks = append(blocks, v.(*BlockContainer).Block())
	}
	return blocks
}

// IsKnownBlock checks whether block is known.
func (f *Forks) IsKnownBlock(blockID flow.Identifier) bool {
	_, hasBlock := f.forest.GetVertex(blockID)
	return hasBlock
}

// IsProcessingNeeded determines whether the given block needs processing,
// based on the block's view and hash.
// Returns false if any of the following conditions applies
//   - block view is _below_ the most recently finalized block
//   - the block already exists in the consensus state
//
// UNVALIDATED: expects block to pass Forks.EnsureBlockIsValidExtension(block)
func (f *Forks) IsProcessingNeeded(block *model.Block) bool {
	if block.View < f.FinalizedView() || f.IsKnownBlock(block.BlockID) {
		return false
	}
	return true
}

// EnsureBlockIsValidExtension checks that the given block is a valid extension to the tree
// of blocks already stored (no state modifications). Specifically, the following conditions
// are enforced, which are critical to the correctness of Forks:
//
//  1. If a block with the same ID is already stored, their views must be identical.
//  2. The block's view must be strictly larger than the view of its parent.
//  3. The parent must already be stored (or below the pruning height).
//
// Exclusions to these rules (by design):
// Let W denote the view of block's parent (i.e. W := block.QC.View) and F the latest
// finalized view.
//
//	  (i) If block.View < F, adding the block would be a no-op. Such blocks are considered
//	      compatible (principle of vacuous truth), i.e. we skip checking 1, 2, 3.
//	 (ii) If block.View == F, we do not inspect the QC / parent at all (skip 2 and 3).
//	      This exception is important for compatability with genesis or spork-root blocks,
//	      which do not contain a QC.
//	(iii) If block.View > F, but block.QC.View < F the parent has already been pruned. In
//	      this case, we omit rule 3. (principle of vacuous truth applied to the parent)
//
// We assume that all blocks are fully verified. A valid block must satisfy all consistency
// requirements; otherwise we have a bug in the compliance layer.
//
// Error returns:
//   - model.MissingBlockError if the parent of the input proposal does not exist in the
//     forest (but is above the pruned view). Represents violation of condition 3.
//   - model.InvalidBlockError if the block violates condition 1. or 2.
//   - generic error in case of unexpected bug or internal state corruption
func (f *Forks) EnsureBlockIsValidExtension(block *model.Block) error {
	if block.View < f.forest.LowestLevel { // exclusion (i)
		return nil
	}

	// LevelledForest enforces conditions 1. and 2. including the respective exclusions (ii) and (iii).
	blockContainer := ToBlockContainer2(block)
	err := f.forest.VerifyVertex(blockContainer)
	if err != nil {
		if forest.IsInvalidVertexError(err) {
			return model.NewInvalidBlockErrorf(block, "not a valid vertex for block tree: %w", err)
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
	// For a block whose parent is _not_ below the pruning height, we expect the parent to be known.
	if _, isParentKnown := f.forest.GetVertex(block.QC.BlockID); !isParentKnown { // missing parent
		return model.MissingBlockError{
			View:    block.QC.View,
			BlockID: block.QC.BlockID,
		}
	}
	return nil
}

// AddCertifiedBlock appends the given certified block to the tree of pending
// blocks and updates the latest finalized block (if finalization progressed).
// Unless the parent is below the pruning threshold (latest finalized view), we
// require that the parent is already stored in Forks. Calling this method with
// previously processed blocks leaves the consensus state invariant (though,
// it will potentially cause some duplicate processing).
//
// Possible error returns:
//   - model.MissingBlockError if the parent does not exist in the forest (but is above
//     the pruned view). From the perspective of Forks, this error is benign (no-op).
//   - model.InvalidBlockError if the block is invalid (see `Forks.EnsureBlockIsValidExtension`
//     for details). From the perspective of Forks, this error is benign (no-op). However, we
//     assume all blocks are fully verified, i.e. they should satisfy all consistency
//     requirements. Hence, this error is likely an indicator of a bug in the compliance layer.
//   - model.ByzantineThresholdExceededError if conflicting QCs or conflicting finalized
//     blocks have been detected (violating a foundational consensus guarantees). This
//     indicates that there are 1/3+ Byzantine nodes (weighted by stake) in the network,
//     breaking the safety guarantees of HotStuff (or there is a critical bug / data
//     corruption). Forks cannot recover from this exception.
//   - All other errors are potential symptoms of bugs or state corruption.
func (f *Forks) AddCertifiedBlock(certifiedBlock *model.CertifiedBlock) error {
	if !f.IsProcessingNeeded(certifiedBlock.Block) {
		return nil
	}

	// Check proposal for byzantine evidence, store it and emit `OnBlockIncorporated` notification.
	// Note: `checkForByzantineEvidence` only inspects the block, but _not_ its certifying QC. Hence,
	// we have to additionally check here, whether the certifying QC conflicts with any known QCs.
	err := f.checkForByzantineEvidence(certifiedBlock.Block)
	if err != nil {
		return fmt.Errorf("cannot check for Byzantine evidence in certified block %v: %w", certifiedBlock.Block.BlockID, err)
	}
	err = f.checkForConflictingQCs(certifiedBlock.CertifyingQC)
	if err != nil {
		return fmt.Errorf("certifying QC for block %v failed check for conflicts: %w", certifiedBlock.Block.BlockID, err)
	}
	f.forest.AddVertex(ToBlockContainer2(certifiedBlock.Block))
	f.notifier.OnBlockIncorporated(certifiedBlock.Block)

	// Update finality status:
	err = f.checkForAdvancingFinalization(certifiedBlock)
	if err != nil {
		return fmt.Errorf("updating finalization failed: %w", err)
	}
	return nil
}

// AddValidatedBlock appends the validated block to the tree of pending
// blocks and updates the latest finalized block (if applicable). Unless the parent is
// below the pruning threshold (latest finalized view), we require that the parent is
// already stored in Forks. Calling this method with previously processed blocks
// leaves the consensus state invariant (though, it will potentially cause some
// duplicate processing).
// Notes:
//   - Method `AddCertifiedBlock(..)` should be used preferably, if a QC certifying
//     `block` is already known. This is generally the case for the consensus follower.
//     Method `AddValidatedBlock` is intended for active consensus participants, which fully
//     validate blocks (incl. payload), i.e. QCs are processed as part of validated proposals.
//
// Possible error returns:
//   - model.MissingBlockError if the parent does not exist in the forest (but is above
//     the pruned view). From the perspective of Forks, this error is benign (no-op).
//   - model.InvalidBlockError if the block is invalid (see `Forks.EnsureBlockIsValidExtension`
//     for details). From the perspective of Forks, this error is benign (no-op). However, we
//     assume all blocks are fully verified, i.e. they should satisfy all consistency
//     requirements. Hence, this error is likely an indicator of a bug in the compliance layer.
//   - model.ByzantineThresholdExceededError if conflicting QCs or conflicting finalized
//     blocks have been detected (violating a foundational consensus guarantees). This
//     indicates that there are 1/3+ Byzantine nodes (weighted by stake) in the network,
//     breaking the safety guarantees of HotStuff (or there is a critical bug / data
//     corruption). Forks cannot recover from this exception.
//   - All other errors are potential symptoms of bugs or state corruption.
func (f *Forks) AddValidatedBlock(proposal *model.Block) error {
	if !f.IsProcessingNeeded(proposal) {
		return nil
	}

	// Check proposal for byzantine evidence, store it and emit `OnBlockIncorporated` notification:
	err := f.checkForByzantineEvidence(proposal)
	if err != nil {
		return fmt.Errorf("cannot check Byzantine evidence for block %v: %w", proposal.BlockID, err)
	}
	f.forest.AddVertex(ToBlockContainer2(proposal))
	f.notifier.OnBlockIncorporated(proposal)

	// Update finality status: In the implementation, our notion of finality is based on certified blocks.
	// The certified parent essentially combines the parent, with the QC contained in block, to drive finalization.
	parent, found := f.GetBlock(proposal.QC.BlockID)
	if !found {
		// Not finding the parent means it is already pruned; hence this block does not change the finalization state.
		return nil
	}
	certifiedParent, err := model.NewCertifiedBlock(parent, proposal.QC)
	if err != nil {
		return fmt.Errorf("mismatching QC with parent (corrupted Forks state):%w", err)
	}
	err = f.checkForAdvancingFinalization(&certifiedParent)
	if err != nil {
		return fmt.Errorf("updating finalization failed: %w", err)
	}
	return nil
}

// checkForByzantineEvidence inspects whether the given `block` together with the already
// known information yields evidence of byzantine behaviour. Furthermore, the method enforces
// that `block` is a valid extension of the tree of pending blocks. If the block is a double
// proposal, we emit an `OnBlockIncorporated` notification. Though, provided the block is a
// valid extension of the block tree by itself, it passes this method without an error.
//
// Possible error returns:
//   - model.MissingBlockError if the parent does not exist in the forest (but is above
//     the pruned view). From the perspective of Forks, this error is benign (no-op).
//   - model.InvalidBlockError if the block is invalid (see `Forks.EnsureBlockIsValidExtension`
//     for details). From the perspective of Forks, this error is benign (no-op). However, we
//     assume all blocks are fully verified, i.e. they should satisfy all consistency
//     requirements. Hence, this error is likely an indicator of a bug in the compliance layer.
//   - model.ByzantineThresholdExceededError if conflicting QCs have been detected.
//     Forks cannot recover from this exception.
//   - All other errors are potential symptoms of bugs or state corruption.
func (f *Forks) checkForByzantineEvidence(block *model.Block) error {
	err := f.EnsureBlockIsValidExtension(block)
	if err != nil {
		return fmt.Errorf("consistency check on block failed: %w", err)
	}
	err = f.checkForConflictingQCs(block.QC)
	if err != nil {
		return fmt.Errorf("checking QC for conflicts failed: %w", err)
	}
	f.checkForDoubleProposal(block)
	return nil
}

// checkForConflictingQCs checks if QC conflicts with a stored Quorum Certificate.
// In case a conflicting QC is found, an ByzantineThresholdExceededError is returned.
// Two Quorum Certificates q1 and q2 are defined as conflicting iff:
//
//	q1.View == q2.View AND q1.BlockID ≠ q2.BlockID
//
// This means there are two Quorums for conflicting blocks at the same view.
// Per 'Observation 1' from the Jolteon paper https://arxiv.org/pdf/2106.10362v1.pdf,
// two conflicting QCs can exist if and only if the Byzantine threshold is exceeded.
// Error returns:
//   - model.ByzantineThresholdExceededError if conflicting QCs have been detected.
//     Forks cannot recover from this exception.
//   - All other errors are potential symptoms of bugs or state corruption.
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
				otherChild := otherChildren.NextVertex().(*BlockContainer).Block()
				conflictingQC := otherChild.QC
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
func (f *Forks) checkForDoubleProposal(block *model.Block) {
	it := f.forest.GetVerticesAtLevel(block.View)
	for it.HasNext() {
		otherVertex := it.NextVertex() // by construction, must have same view as block
		otherBlock := otherVertex.(*BlockContainer).Block()
		if block.BlockID != otherBlock.BlockID {
			f.notifier.OnDoubleProposeDetected(block, otherBlock)
		}
	}
}

// checkForAdvancingFinalization checks whether observing certifiedBlock leads to progress of
// finalization. This function should be called every time a new block is added to Forks. If the new
// block is the head of a 2-chain satisfying the finalization rule, we update `Forks.finalityProof` to
// the new latest finalized block. Calling this method with previously-processed blocks leaves the
// consensus state invariant.
// UNVALIDATED: assumes that relevant block properties are consistent with previous blocks
// Error returns:
//   - model.MissingBlockError if the parent does not exist in the forest (but is above
//     the pruned view). From the perspective of Forks, this error is benign (no-op).
//   - model.ByzantineThresholdExceededError in case we detect a finalization fork (violating
//     a foundational consensus guarantee). This indicates that there are 1/3+ Byzantine nodes
//     (weighted by stake) in the network, breaking the safety guarantees of HotStuff (or there
//     is a critical bug / data corruption). Forks cannot recover from this exception.
//   - generic error in case of unexpected bug or internal state corruption
func (f *Forks) checkForAdvancingFinalization(certifiedBlock *model.CertifiedBlock) error {
	// We prune all blocks in forest which are below the most recently finalized block.
	// Hence, we have a pruned ancestry if and only if either of the following conditions applies:
	//    (a) If a block's parent view (i.e. block.QC.View) is below the most recently finalized block.
	//    (b) If a block's view is equal to the most recently finalized block.
	// Caution:
	// * Under normal operation, case (b) is covered by the logic for case (a)
	// * However, the existence of a genesis block requires handling case (b) explicitly:
	//   The root block is specified and trusted by the node operator. If the root block is the
	//   genesis block, it might not contain a QC pointing to a parent (as there is no parent).
	//   In this case, condition (a) cannot be evaluated.
	lastFinalizedView := f.FinalizedView()
	if (certifiedBlock.View() <= lastFinalizedView) || (certifiedBlock.Block.QC.View < lastFinalizedView) {
		// Repeated blocks are expected during normal operations. We enter this code block if and only
		// if the parent's view is _below_ the last finalized block. It is straight forward to show:
		// Lemma: Let B be a block whose 2-chain reaches beyond the last finalized block
		//        => B will not update the locked or finalized block
		return nil
	}

	// retrieve parent; always expected to succeed, because we passed the checks above
	qcForParent := certifiedBlock.Block.QC
	parentVertex, parentBlockKnown := f.forest.GetVertex(qcForParent.BlockID)
	if !parentBlockKnown {
		return model.MissingBlockError{View: qcForParent.View, BlockID: qcForParent.BlockID}
	}
	parentBlock := parentVertex.(*BlockContainer).Block()

	// Note: we assume that all stored blocks pass Forks.EnsureBlockIsValidExtension(block);
	//       specifically, that Proposal's ViewNumber is strictly monotonically
	//       increasing which is enforced by LevelledForest.VerifyVertex(...)
	// We denote:
	//  * a DIRECT 1-chain as '<-'
	//  * a general 1-chain as '<~' (direct or indirect)
	// Jolteon's rule for finalizing `parentBlock` is
	//     parentBlock <- Block <~ certifyingQC    (i.e. a DIRECT 1-chain PLUS any 1-chain)
	//                   ╰─────────────────────╯
	//                       certifiedBlock
	// Hence, we can finalize `parentBlock` as head of a 2-chain,
	// if and only if `Block.View` is exactly 1 higher than the view of `parentBlock`
	if parentBlock.View+1 != certifiedBlock.View() {
		return nil
	}

	// `parentBlock` is now finalized:
	//  * While Forks is single-threaded, there is still the possibility of reentrancy. Specifically, the
	//    consumers of our finalization events are served by the goroutine executing Forks. It is conceivable
	//    that a consumer might access Forks and query the latest finalization proof. This would be legal, if
	//    the component supplying the goroutine to Forks also consumes the notifications.
	//  * Therefore, for API safety, we want to first update Fork's `finalityProof` before we emit any notifications.

	// Advancing finalization step (i): we collect all blocks for finalization (no notifications are emitted)
	blocksToBeFinalized, err := f.collectBlocksForFinalization(qcForParent)
	if err != nil {
		return fmt.Errorf("advancing finalization to block %v from view %d failed: %w", qcForParent.BlockID, qcForParent.View, err)
	}

	// Advancing finalization step (ii): update `finalityProof` and prune `LevelledForest`
	f.finalityProof = &hotstuff.FinalityProof{Block: parentBlock, CertifiedChild: *certifiedBlock}
	err = f.forest.PruneUpToLevel(f.FinalizedView())
	if err != nil {
		return fmt.Errorf("pruning levelled forest failed unexpectedly: %w", err)
	}

	// Advancing finalization step (iii): iterate over the blocks from (i) and emit finalization events
	for _, b := range blocksToBeFinalized {
		// first notify other critical components about finalized block - all errors returned here are fatal exceptions
		err = f.finalizationCallback.MakeFinal(b.BlockID)
		if err != nil {
			return fmt.Errorf("finalization error in other component: %w", err)
		}

		// notify less important components about finalized block
		f.notifier.OnFinalizedBlock(b)
	}
	return nil
}

// collectBlocksForFinalization collects and returns all newly finalized blocks up to (and including)
// the block pointed to by `qc`. The blocks are listed in order of increasing height.
// Error returns:
//   - model.ByzantineThresholdExceededError in case we detect a finalization fork (violating
//     a foundational consensus guarantee). This indicates that there are 1/3+ Byzantine nodes
//     (weighted by stake) in the network, breaking the safety guarantees of HotStuff (or there
//     is a critical bug / data corruption). Forks cannot recover from this exception.
//   - generic error in case of bug or internal state corruption
func (f *Forks) collectBlocksForFinalization(qc *flow.QuorumCertificate) ([]*model.Block, error) {
	lastFinalized := f.FinalizedBlock()
	if qc.View < lastFinalized.View {
		return nil, model.ByzantineThresholdExceededError{Evidence: fmt.Sprintf(
			"finalizing block with view %d which is lower than previously finalized block at view %d",
			qc.View, lastFinalized.View,
		)}
	}
	if qc.View == lastFinalized.View { // no new blocks to be finalized
		return nil, nil
	}

	// Collect all blocks that are pending finalization in slice. While we crawl the blocks starting
	// from the newest finalized block backwards (decreasing views), we would like to return them in
	// order of _increasing_ view. Therefore, we fill the slice starting with the highest index.
	l := qc.View - lastFinalized.View // l is an upper limit to the number of blocks that can be maximally finalized
	blocksToBeFinalized := make([]*model.Block, l)
	for qc.View > lastFinalized.View {
		b, ok := f.GetBlock(qc.BlockID)
		if !ok {
			return nil, fmt.Errorf("failed to get block (view=%d, blockID=%x) for finalization", qc.View, qc.BlockID)
		}
		l--
		blocksToBeFinalized[l] = b
		qc = b.QC // move to parent
	}
	// Now, `l` is the index where we stored the oldest block that should be finalized. Note that `l`
	// might be larger than zero, if some views have no finalized blocks. Hence, `blocksToBeFinalized`
	// might start with nil entries, which we remove:
	blocksToBeFinalized = blocksToBeFinalized[l:]

	// qc should now point to the latest finalized block. Otherwise, the
	// consensus committee is compromised (or we have a critical internal bug).
	if qc.View < lastFinalized.View {
		return nil, model.ByzantineThresholdExceededError{Evidence: fmt.Sprintf(
			"finalizing block with view %d which is lower than previously finalized block at view %d",
			qc.View, lastFinalized.View,
		)}
	}
	if qc.View == lastFinalized.View && lastFinalized.BlockID != qc.BlockID {
		return nil, model.ByzantineThresholdExceededError{Evidence: fmt.Sprintf(
			"finalizing blocks with view %d at conflicting forks: %x and %x",
			qc.View, qc.BlockID, lastFinalized.BlockID,
		)}
	}

	return blocksToBeFinalized, nil
}
