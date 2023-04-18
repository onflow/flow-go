package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// FinalityProof represents a finality proof for a Block. By convention, a FinalityProof
// is immutable. Finality in Jolteon/HotStuff is determined by the 2-chain rule:
//
//	There exists a _certified_ block C, such that Block.View + 1 = C.View
type FinalityProof struct {
	Block          *model.Block
	CertifiedChild model.CertifiedBlock
}

// Forks maintains an in-memory data-structure of all blocks whose view-number is larger or equal to
// the latest finalized block. The latest finalized block is defined as the finalized block with the largest view number.
// When adding blocks, Forks automatically updates its internal state (including finalized blocks).
// Furthermore, blocks whose view number is smaller than the latest finalized block are pruned automatically.
//
// PREREQUISITES:
// Forks expects that only blocks are added that can be connected to its latest finalized block
// (without missing interim ancestors). If this condition is violated, Forks will raise an error
// and ignore the block.
type Forks interface {

	// GetBlocksForView returns all known blocks for the given view
	GetBlocksForView(view uint64) []*model.Block

	// GetBlock returns (BlockProposal, true) if the block with the specified
	// id was found and (nil, false) otherwise.
	GetBlock(blockID flow.Identifier) (*model.Block, bool)

	// FinalizedView returns the largest view number where a finalized block is known
	FinalizedView() uint64

	// FinalizedBlock returns the finalized block with the largest view number
	FinalizedBlock() *model.Block

	// FinalityProof returns the latest finalized block and a certified child from
	// the subsequent view, which proves finality.
	// CAUTION: method returns (nil, false), when Forks has not yet finalized any
	// blocks beyond the finalized root block it was initialized with.
	FinalityProof() (*FinalityProof, bool)

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
	AddValidatedBlock(proposal *model.Block) error

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
	AddCertifiedBlock(certifiedBlock *model.CertifiedBlock) error
}
