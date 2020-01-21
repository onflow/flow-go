package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// Forks encapsulated Finalization Logic and ForkChoice rule in one component.
// Forks maintains an in-memory data-structure of all blocks whose view-number is larger or equal to
// the latest finalized block. The latest finalized block is defined as the finalized block with the largest view number.
// When adding blocks, Forks automatically updates its internal state (including finalized blocks).
// Furthermore, blocks whose view number is smaller than the latest finalized block are pruned automatically.
//
// PREREQUISITES:
// * From the view-point of Forks, a block B is identified by the pair (B.View, B.blockMRH)
// * Forks expects that only blocks are added that can be connected to its latest finalized block
//   (without missing interim ancestors). If this condition is violated, Forks will raise an error
//   and irgnore the block.
type Forks interface {

	// GetBlockForView returns the BlockProposal at the given view number if exists.
	// When there is multiple proposals for the same view, Forks will only return one.
	GetBlockForView(view uint64) (*types.BlockProposal, bool)

	// GetBlock returns (BlockProposal, true) if the block with view and blockMRH was found (both values need to match)
	// or (nil, false) otherwise.
	GetBlock(view uint64, blockMRH []byte) (*types.BlockProposal, bool)

	// FinalizedView returns the largest view number where a finalized block is known
	FinalizedView() uint64

	// FinalizedBlock returns the finalized block with the largest view number
	FinalizedBlock() *types.BlockProposal

	// IsSafeNode returns true if block is safe to vote for
	// (according to the definition in https://arxiv.org/abs/1803.05069v6).
	// Returns false for unknown blocks.
	IsSafeNode(block *types.BlockProposal) bool

	// AddBlock adds the block to Forks. This might cause an update of the finalized block
	// and pruning of older blocks.
	// Handles duplicated addition of blocks (at the potential cost of additional computation time).
	// PREREQUISITE:
	// Forks must be able to connect `block` to its latest finalized block
	// (without missing interim ancestors). Otherwise, an error is raised.
	// When the new block causes the conflicting finalized blocks, it will return
	// FinalizationFatalError
	AddBlock(block *types.BlockProposal) error

	// AddQC adds a quorum certificate to Forks.
	// Might error in case the block referenced by the QuorumCertificate is unknown.
	AddQC(*types.QuorumCertificate) error

	// MakeForkChoice prompts the ForkChoice to generate a fork choice.
	// The fork choice is a qc that should be used for building the primaries block
	MakeForkChoice(viewNumber uint64) *types.QuorumCertificate
}
