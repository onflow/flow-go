package hotstuff

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
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
	ForksReader

	// AddBlock adds the block to Forks. This might cause an update of the finalized block
	// and pruning of older blocks.
	// Handles duplicated addition of blocks (at the potential cost of additional computation time).
	// PREREQUISITE:
	// Forks must be able to connect `block` to its latest finalized block
	// (without missing interim ancestors). Otherwise, an error is raised.
	// When the new block causes the conflicting finalized blocks, it will return
	// Might error with ByzantineThresholdExceededError (e.g. if finalizing conflicting forks)
	AddBlock(block *model.Block) error

	// AddQC adds a quorum certificate to Forks.
	// Will error in case the block referenced by the qc is unknown.
	// Might error with ByzantineThresholdExceededError (e.g. if two conflicting QCs for the
	// same view are found)
	AddQC(qc *model.QuorumCertificate) error

	// MakeForkChoice prompts the ForkChoice to generate a fork choice for the
	// current view `curView`. The fork choice is a qc that should be used for
	// building the primaries block.
	// It returns a qc and the block that the qc is pointing to.
	//
	// PREREQUISITE:
	// ForkChoice cannot generate ForkChoices retroactively for past views.
	// If used correctly, MakeForkChoice should only ever have processed QCs
	// whose view is smaller than curView, for the following reason:
	// Processing a QC with view v should result in the PaceMaker being in
	// view v+1 or larger. Hence, given that the current View is curView,
	// all QCs should have view < curView.
	// To prevent accidental misusage, ForkChoices will error if `curView`
	// is smaller than the view of any qc ForkChoice has seen.
	// Note that tracking the view of the newest qc is for safety purposes
	// and _independent_ of the fork-choice rule.
	MakeForkChoice(curView uint64) (*model.QuorumCertificate, *model.Block, error)
}

// ForksReader only reads the forks' state
type ForksReader interface {

	// IsSafeBlock returns true if block is safe to vote for
	// (according to the definition in https://arxiv.org/abs/1803.05069v6).
	//
	// In the current architecture, the block is stored _before_ evaluating its safety.
	// Consequently, IsSafeBlock accepts only known, valid blocks. Should a block be
	// unknown (not previously added to Forks) or violate some consistency requirements,
	// IsSafeBlock errors. All errors are fatal.
	IsSafeBlock(block *model.Block) bool

	// GetBlocksForView returns all BlockProposals at the given view number.
	GetBlocksForView(view uint64) []*model.Block

	// GetBlock returns (BlockProposal, true) if the block with the specified
	// id was found (nil, false) otherwise.
	GetBlock(id flow.Identifier) (*model.Block, bool)

	// FinalizedView returns the largest view number where a finalized block is known
	FinalizedView() uint64

	// FinalizedBlock returns the finalized block with the largest view number
	FinalizedBlock() *model.Block
}
