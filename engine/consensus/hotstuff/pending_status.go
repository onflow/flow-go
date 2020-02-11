package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type PendingVotes struct {
	// maps block ID to pending status for that block
	votes map[flow.Identifier]*PendingStatus
}

// PendingStatus keeps track of pending votes for the same block
type PendingStatus struct {
	// When receiving missing block, first received votes will be accumulated
	orderedVotes []*types.Vote
	// For avoiding duplicate votes
	voteMap map[flow.Identifier]struct{}
}

func (pv *PendingVotes) AddVote(vote *types.Vote) {
	status, exists := pv.votes[vote.BlockID]
	if !exists {
		status = NewPendingStatus()
		pv.votes[vote.BlockID] = status
	}
	status.AddVote(vote)
}

func (ps *PendingStatus) AddVote(vote *types.Vote) {
	_, exists := ps.voteMap[vote.ID()]
	if exists {
		return
	}
	ps.voteMap[vote.ID()] = struct{}{}
	ps.orderedVotes = append(ps.orderedVotes, vote)
}

func NewPendingVotes() *PendingVotes {
	return &PendingVotes{votes: make(map[flow.Identifier]*PendingStatus)}
}

func NewPendingStatus() *PendingStatus {
	return &PendingStatus{voteMap: make(map[flow.Identifier]struct{})}
}
