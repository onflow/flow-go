package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// VoteAggregator aggregates votes and produces quorum certificates.
type VoteAggregator interface {

	// StorePendingVote is used to store a vote for a block for which we don't
	// have a block yet.
	StorePendingVote(vote *model.Vote) (bool, error)

	// StoreVoteAndBuildQC will store a vote and build the QC related to the
	// voted upon block if enough votes can be accumulated.
	StoreVoteAndBuildQC(vote *model.Vote, block *model.Block) (*flow.QuorumCertificate, bool, error)

	// StoreProposerVote stores the vote of the proposer of the block and is
	// used to separate vote and block handling.
	StoreProposerVote(vote *model.Vote) bool

	// BuildQCOnReceivedBlock will try to build a QC for the received block in
	// case enough votes can be accumulated for it.
	BuildQCOnReceivedBlock(block *model.Block) (*flow.QuorumCertificate, bool, error)

	// PruneByView will remove any data held for the provided view.
	PruneByView(view uint64)
}
