package integration

import (
	"math/rand"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
)

type VoteFilter func(*model.Vote) bool

func BlockNoVotes(*model.Vote) bool {
	return false
}

func BlockAllVotes(*model.Vote) bool {
	return true
}

func BlockVoteRatio(ratio float64) VoteFilter {
	return func(*model.Vote) bool {
		return rand.Float64() <= ratio
	}
}

func BlockVotesBy(voterID flow.Identifier) VoteFilter {
	return func(vote *model.Vote) bool {
		return vote.SignerID == voterID
	}
}

type ProposalFilter func(*model.Proposal) bool

func BlockNoProposals(*model.Proposal) bool {
	return false
}

func BlockAllProposals(*model.Proposal) bool {
	return true
}

func BlockProposalRatio(ratio float64) ProposalFilter {
	return func(*model.Proposal) bool {
		return rand.Float64() <= ratio
	}
}

func BlockProposalsBy(proposerID flow.Identifier) ProposalFilter {
	return func(proposal *model.Proposal) bool {
		return proposal.Block.ProposerID == proposerID
	}
}
