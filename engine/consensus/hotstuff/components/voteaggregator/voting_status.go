package voteaggregator

import "github.com/dapperlabs/flow-go/engine/consensus/HotStuff/types"

type VotingStatus struct {
	thresholdStake   uint64
	accumulatedStake uint64
	validVotes       map[string]*types.Vote
}

func (vs *VotingStatus) canBuildQC() bool {
	return vs.accumulatedStake >= vs.thresholdStake
}
