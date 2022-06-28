package forks

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// Finalizer is responsible for block finalization.
type Finalizer interface {
	VerifyProposal(proposal *model.Proposal) error
	IsSafeBlock(*model.Block) bool
	AddProposal(proposal *model.Proposal) error
	GetProposal(blockID flow.Identifier) (*model.Proposal, bool)
	GetProposalsForView(view uint64) []*model.Proposal
	FinalizedBlock() *model.Block
	LockedBlock() *model.Block
}
