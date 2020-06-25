package voteaggregator

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
)

// PendingVotes stores all the pending votes for different block proposals
type PendingVotes struct {
	// maps block ID to pending status for that block
	votes map[flow.Identifier]*PendingStatus
}

// PendingStatus keeps track of pending votes for the same block
type PendingStatus struct {
	// When receiving missing block, first received votes will be accumulated
	orderedVotes []*model.Vote
	// For avoiding duplicate votes
	voteMap map[flow.Identifier]struct{}
}

// AddVote adds a vote as a pending vote
// returns true if it can be added to a PendingStatus successfully
// returns false otherwise
func (pv *PendingVotes) AddVote(vote *model.Vote) bool {
	status, exists := pv.votes[vote.BlockID]
	if !exists {
		status = NewPendingStatus()
		pv.votes[vote.BlockID] = status
	}
	return status.AddVote(vote)
}

// AddVote adds a vote as a pending vote
// returns false if it has been added before
// returns true otherwise
func (ps *PendingStatus) AddVote(vote *model.Vote) bool {
	_, exists := ps.voteMap[vote.ID()]
	if exists {
		return false
	}
	ps.voteMap[vote.ID()] = struct{}{}
	ps.orderedVotes = append(ps.orderedVotes, vote)
	return true
}

// NewPendingVotes creates a PendingVotes instance
func NewPendingVotes() *PendingVotes {
	return &PendingVotes{votes: make(map[flow.Identifier]*PendingStatus)}
}

// NewPendingStatus creates a PendingStatus instance
func NewPendingStatus() *PendingStatus {
	return &PendingStatus{voteMap: make(map[flow.Identifier]struct{})}
}
