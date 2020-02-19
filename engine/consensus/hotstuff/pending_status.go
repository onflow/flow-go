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

func (pv *PendingVotes) AddVote(vote *types.Vote) bool {
	status, exists := pv.votes[vote.BlockID]
	if !exists {
		status = NewPendingStatus()
		pv.votes[vote.BlockID] = status
	}
	return status.AddVote(vote)
}

func (ps *PendingStatus) AddVote(vote *types.Vote) bool {
	_, exists := ps.voteMap[vote.ID()]
	if exists {
		return false
	}
	ps.voteMap[vote.ID()] = struct{}{}
	ps.orderedVotes = append(ps.orderedVotes, vote)
	return true
}

func NewPendingVotes() *PendingVotes {
	return &PendingVotes{votes: make(map[flow.Identifier]*PendingStatus)}
}

func NewPendingStatus() *PendingStatus {
	return &PendingStatus{voteMap: make(map[flow.Identifier]struct{})}
}
