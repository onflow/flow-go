package voteaggregator

import (
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type VotingStatus struct {
	thresholdStake   uint64
	accumulatedStake uint64
	voteSender       flow.Identity
	validVotes       map[string]*types.Vote
}

func NewVotingStatus(thresholdStake uint64, voteSender flow.Identity) *VotingStatus {
	return &VotingStatus{
		thresholdStake: thresholdStake,
		voteSender:     voteSender,
	}
}

func (vs *VotingStatus) AddVote(vote *types.Vote) {
	vs.validVotes[vote.Hash()] = vote
	vs.accumulatedStake += vs.voteSender.Stake
}

func (vs *VotingStatus) canBuildQC() bool {
	return vs.accumulatedStake >= vs.thresholdStake
}
