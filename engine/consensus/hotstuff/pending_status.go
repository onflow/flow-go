package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

// PendingStatus keeps track of pending votes
type PendingStatus struct {
	// When receiving missing block, first received votes will be accumulated
	orderedVotes []*types.Vote
	// For avoiding duplicate votes
	voteMap map[flow.Identifier]struct{}
}

func (ps *PendingStatus) AddVote(vote *types.Vote) {
	_, exists := ps.voteMap[vote.ID()]
	if exists {
		return
	}
	ps.voteMap[vote.ID()] = struct{}{}
	ps.orderedVotes = append(ps.orderedVotes, vote)
}

func NewPendingStatus() *PendingStatus {
	return &PendingStatus{
		voteMap: make(map[flow.Identifier]struct{}),
	}
}
