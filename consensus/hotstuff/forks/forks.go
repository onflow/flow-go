package forks

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// Forks is a tiny wrapper over Finalizer which implements hotstuff.Forks interface
// and performs state lookups and safe insertion of blocks
type Forks struct {
	finalizer Finalizer
}

var _ hotstuff.Forks = (*Forks)(nil)

// New creates a Forks instance
func New(finalizer Finalizer) *Forks {
	return &Forks{
		finalizer: finalizer,
	}
}

// GetProposalsForView returns all BlockProposals at the given view number.
func (f *Forks) GetProposalsForView(view uint64) []*model.Proposal {
	return f.finalizer.GetProposalsForView(view)
}

// GetProposal returns (BlockProposal, true) if the block with the specified
// id was found (nil, false) otherwise.
func (f *Forks) GetProposal(id flow.Identifier) (*model.Proposal, bool) {
	return f.finalizer.GetProposal(id)
}

// FinalizedBlock returns the latest finalized block
func (f *Forks) FinalizedBlock() *model.Block {
	return f.finalizer.FinalizedBlock()
}

// FinalizedView returns the view of the latest finalized block
func (f *Forks) FinalizedView() uint64 {
	return f.finalizer.FinalizedBlock().View
}

// AddProposal passes the block proposal to the finalizer for finalization
func (f *Forks) AddProposal(proposal *model.Proposal) error {
	if err := f.finalizer.VerifyProposal(proposal); err != nil {
		// technically, this not strictly required. However, we leave this as a sanity check for now
		return fmt.Errorf("cannot add invalid proposal to Forks: %w", err)
	}
	err := f.finalizer.AddProposal(proposal)
	if err != nil {
		return fmt.Errorf("error storing proposal in Forks: %w", err)
	}

	return nil
}
