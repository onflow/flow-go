package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/signature"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type VoteAggregator struct {
	pendingVotes map[string][]*types.Vote
	createdQC    map[string]*types.QuorumCertificate
	viewState    ViewState
	sigProvider  signature.SigProvider
}

// StorePendingVote stores the vote as a pending vote assuming the caller has checked that the voting
// block is currently missing.
// Note: Validations on these pending votes will be postponded until the block has been received.
func (va *VoteAggregator) StorePendingVote(v *types.Vote) {
	panic("TODO")
}

// StoreVoteAndBuildQC stores the vote assuming the caller has checked that the voting block is incorporated,
// and returns a QC if there are votes with enough stakes.
// The VoteAggregator builds a QC as soon as the number of votes allow this.
// While subsequent votes (past the required threshold) are not included in the QC anymore,
// VoteAggregator ALWAYS returns the same QC as the one returned before.
func (va *VoteAggregator) StoreVoteAndBuildQC(v *types.Vote, b *types.BlockProposal) (*types.QuorumCertificate, bool) {
	panic("TODO")
}

// BuildQCForBlockProposal will attempt to build a QC for the given block proposal when there are votes
// with enough stakes.
// VoteAggregator ALWAYS returns the same QC as the one returned before.
func (va *VoteAggregator) BuildQCForBlockProposal(b *types.BlockProposal) (*types.QuorumCertificate, bool) {
	panic("TODO")
}

// PruneByView will delete all votes equal or below to the given view, as well as related indexes.
func (va *VoteAggregator) PruneByView(view uint64) {
	panic("TODO")
}
