package integration

import (
	"math/rand"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// VoteFilter is a filter function for dropping Votes.
// Return value `true` implies that the the given Vote should be
// dropped, while `false` indicates that the Vote should be received.
type VoteFilter func(*model.Vote) bool

func BlockNoVotes(*model.Vote) bool {
	return false
}

func BlockAllVotes(*model.Vote) bool {
	return true
}

// BlockVoteRandomly drops votes randomly with a probability of `dropProbability` ∈ [0,1]
func BlockVoteRandomly(dropProbability float64) VoteFilter {
	return func(*model.Vote) bool {
		return rand.Float64() < dropProbability
	}
}

func BlockVotesBy(voterID flow.Identifier) VoteFilter {
	return func(vote *model.Vote) bool {
		return vote.SignerID == voterID
	}
}

// ProposalFilter is a filter function for dropping Proposals.
// Return value `true` implies that the the given Proposal should be
// dropped, while `false` indicates that the Proposal should be received.
type ProposalFilter func(*model.Proposal) bool

func BlockNoProposals(*model.Proposal) bool {
	return false
}

func BlockAllProposals(*model.Proposal) bool {
	return true
}

// BlockProposalRandomly drops proposals randomly with a probability of `dropProbability` ∈ [0,1]
func BlockProposalRandomly(dropProbability float64) ProposalFilter {
	return func(*model.Proposal) bool {
		return rand.Float64() < dropProbability
	}
}

// BlockProposalsBy drops all proposals originating from the specified `proposerID`
func BlockProposalsBy(proposerID flow.Identifier) ProposalFilter {
	return func(proposal *model.Proposal) bool {
		return proposal.Block.ProposerID == proposerID
	}
}

// TimeoutObjectFilter is a filter function for dropping TimeoutObjects.
// Return value `true` implies that the the given TimeoutObject should be
// dropped, while `false` indicates that the TimeoutObject should be received.
type TimeoutObjectFilter func(*model.TimeoutObject) bool

// BlockAllTimeoutObjects always returns `true`, i.e. drops all TimeoutObjects
func BlockAllTimeoutObjects(*model.TimeoutObject) bool {
	return true
}

// BlockNoTimeoutObjects always returns `false`, i.e. it lets all TimeoutObjects pass.
func BlockNoTimeoutObjects(*model.TimeoutObject) bool {
	return false
}
