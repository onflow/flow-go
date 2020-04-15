// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package round

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
)

// Round keeps track of the current consensus state.
type Round struct {
	parent       *flow.Header
	leader       *flow.Identity
	quorum       uint64
	participants flow.IdentityList
	candidate    *flow.Header
	votes        map[flow.Identifier]uint64
}

// New creates a new consensus round.
func New(head *flow.Header, me module.Local, participants flow.IdentityList) (*Round, error) {

	// pick the leader on a fixed round-robin basis
	leader := participants[int(head.Height)%len(participants)]

	// remove ourselves from the participants
	quorum := participants.TotalStake()
	participants = participants.Filter(filter.Not(filter.HasNodeID(me.NodeID())))

	s := &Round{
		parent:       head,
		leader:       leader,
		quorum:       quorum,
		participants: participants,
		votes:        make(map[flow.Identifier]uint64),
	}
	return s, nil
}

// Parent returns the parent of the to-be-formed block.
func (r *Round) Parent() *flow.Header {
	return r.parent
}

// Participants will retrieve cached identities for this round.
func (r *Round) Participants() flow.IdentityList {
	return r.participants
}

// Quorum returns the quorum for a qualified majority in this round.
func (r *Round) Quorum() uint64 {
	return r.quorum
}

// Leader returns the the leader of the current round.
func (r *Round) Leader() *flow.Identity {
	return r.leader
}

// Propose sets the current candidate header.
func (r *Round) Propose(candidate *flow.Header) {
	r.candidate = candidate
}

// Candidate returns the current candidate header.
func (r *Round) Candidate() *flow.Header {
	return r.candidate
}

// Voted checks if the given node has already voted.
func (r *Round) Voted(nodeID flow.Identifier) bool {
	_, ok := r.votes[nodeID]
	return ok
}

// Tally will add the given vote to the node tally.
func (r *Round) Tally(voterID flow.Identifier, stake uint64) {
	r.votes[voterID] = stake
}

// Votes will count the total votes.
func (r *Round) Votes() uint64 {
	var votes uint64
	for _, stake := range r.votes {
		votes += stake
	}
	return votes
}
