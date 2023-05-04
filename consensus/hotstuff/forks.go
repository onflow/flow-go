package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// Forks maintains an in-memory data-structure of all proposals whose view-number is larger or equal to
// the latest finalized block. The latest finalized block is defined as the finalized block with the largest view number.
// When adding blocks, Forks automatically updates its internal state (including finalized blocks).
// Furthermore, blocks whose view number is smaller than the latest finalized block are pruned automatically.
//
// PREREQUISITES:
// Forks expects that only blocks are added that can be connected to its latest finalized block
// (without missing interim ancestors). If this condition is violated, Forks will raise an error
// and ignore the block.
type Forks interface {

	// GetProposalsForView returns all BlockProposals at the given view number.
	GetProposalsForView(view uint64) []*model.Proposal

	// GetProposal returns (BlockProposal, true) if the block with the specified
	// id was found (nil, false) otherwise.
	GetProposal(id flow.Identifier) (*model.Proposal, bool)

	// FinalizedView returns the largest view number where a finalized block is known
	FinalizedView() uint64

	// FinalizedBlock returns the finalized block with the largest view number
	FinalizedBlock() *model.Block

	// NewestView returns the largest view number of all proposals that were added to Forks.
	NewestView() uint64

	// AddProposal adds the block proposal to Forks. This might cause an update of the finalized block
	// and pruning of older blocks.
	// Handles duplicated addition of blocks (at the potential cost of additional computation time).
	// PREREQUISITE:
	// Forks must be able to connect `proposal` to its latest finalized block
	// (without missing interim ancestors). Otherwise, an exception is raised.
	// Expected errors during normal operations:
	//  * model.ByzantineThresholdExceededError - new block results in conflicting finalized blocks
	AddProposal(proposal *model.Proposal) error
}
