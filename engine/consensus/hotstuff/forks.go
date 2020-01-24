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
// Forks expects that only blocks are added that can be connected to its latest finalized block
// (without missing interim ancestors). If this condition is violated, Forks will raise an error
// and ignore the block.
type Forks interface {

	// GetBlocksForView returns all BlockProposals at the given view number.
	GetBlocksForView(view uint64) []*types.BlockProposal

	// GetBlock returns (BlockProposal, true) if the block with the specified
	// id was found (nil, false) otherwise.
	GetBlock(id []byte) (*types.BlockProposal, bool)

	// FinalizedView returns the largest view number where a finalized block is known
	FinalizedView() uint64

	// FinalizedBlock returns the finalized block with the largest view number
	FinalizedBlock() *types.BlockProposal

	// IsSafeNode returns true if block is safe to vote for
	// (according to the definition in https://arxiv.org/abs/1803.05069v6).
	// Returns false for unknown blocks.
	IsSafeBlock(block *types.BlockProposal) bool

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
	// Might error in case the block referenced by the qc is unknown.
	AddQC(qc *types.QuorumCertificate) error

	// MakeForkChoice prompts the ForkChoice to generate a fork choice for the
	// current view `curView`. The fork choice is a qc that should be used for
	// building the primaries block.
	//
	// Error return indicates incorrect usage. Processing a QC with view v
	// should result in the PaceMaker being in view v+1 or larger. Hence, given
	// that the current View is curView, all QCs should have view < curView
	MakeForkChoice(curView uint64) (*types.QuorumCertificate, error)
}
