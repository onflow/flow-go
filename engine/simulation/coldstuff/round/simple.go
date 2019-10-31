// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package round

import (
	"github.com/dapperlabs/flow-go/model/coldstuff"
)

// Simple keeps track of the current consensus state.
type Simple struct {
	candidate *coldstuff.BlockHeader
	votes     map[string]struct{}
}

// NewSimple creates a new consensus cache.
func NewSimple() *Simple {
	s := &Simple{}
	return s
}

// Reset will reset the cache.
func (s *Simple) Reset() {
	s.candidate = nil
	s.votes = make(map[string]struct{})
}

// Propose sets the current candidate header.
func (s *Simple) Propose(candidate *coldstuff.BlockHeader) {
	if s.candidate != nil {
		panic("non-nil candidate proposal")
	}
	s.candidate = candidate
}

// Candidate returns the current candidate header.
func (s *Simple) Candidate() *coldstuff.BlockHeader {
	if s.candidate == nil {
		panic("nil candidate retrieval")
	}
	return s.candidate
}

// Voted checks if the given node has already voted.
func (s *Simple) Voted(nodeID string) bool {
	_, ok := s.votes[nodeID]
	return ok
}

// Tally will add the given vote to the node tally.
func (s *Simple) Tally(voterID string) {
	s.votes[voterID] = struct{}{}
}

// Votes will count the total votes.
func (s *Simple) Votes() uint {
	return uint(len(s.votes))
}
