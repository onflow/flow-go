package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// Crown encapsulated Finalization Logic and ForkChoice rule in one component.
// Crown maintains an in-memory data-structure of all blocks whose view-number is larger or equal to
// the latest finalized block. The latest finalized block is defined as the finalized block with the largest view number.
// When adding blocks, Crown automatically updates its internal state (including finalized blocks).
// Furthermore, blocks whose view number is smaller than the latest finalized block are pruned automatically.
//
// PREREQUISITES:
// * From the view-point of Crown, a block B is identified by the pair (B.View, B.blockMRH)
// * Crown expects that only blocks are added that can be connected to its latest finalized block
//   (without missing interim ancestors). If this condition is violated, Crown will panic (instead of
//   transitioning into an undefined state).
type Crown interface {

	// GetBlocksForView returns the list of all known BlockProposals at the given view number.
	// If none are known, an empty slice is returned.
	GetBlocksForView(view uint64) []*types.BlockProposal

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

	// IsKnownBlock returns true if the consensus reactor knows the specified block
	IsKnownBlock([]byte, uint64) bool

	// IsProcessingNeeded returns true if consensus reactor should process the specified block
	IsProcessingNeeded([]byte, uint64) bool

	// AddBlock adds the block to Crown. This might cause an update of the finalized block
	// and pruning of older blocks.
	// Handles duplicated addition of blocks (at the potential cost of additional computation time).
	// PREREQUISITE:
	// Crown must be able to connect `block` to its latest finalized block (without missing interim ancestors).
	AddBlock(block *types.BlockProposal)

	// AddQC adds a quorum certificate to Crown.
	AddQC(*types.QuorumCertificate)

	// MakeForkChoice prompts the ForkChoice to generate a fork choice.
	// The fork choice is a qc that should be used for building the primaries block
	MakeForkChoice(viewNumber uint64) *types.QuorumCertificate
}
