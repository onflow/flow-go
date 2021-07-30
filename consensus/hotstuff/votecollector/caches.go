package votecollector

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// PendingVotes keeps track of pending votes for the same block.
// PendingVotes is safe to be used my multiple concurrent goroutines.
type PendingVotes struct {
	orderedVotes []*model.Vote                // When receiving missing block, first received votes will be accumulated
	voteMap      map[flow.Identifier]struct{} // For avoiding duplicate votes
	lock         sync.RWMutex
}

// AddVote adds a vote as a pending vote
// returns false if it has been added before
// returns true otherwise
func (ps *PendingVotes) AddVote(vote *model.Vote) bool {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	if _, exists := ps.voteMap[vote.ID()]; !exists {
		ps.voteMap[vote.ID()] = struct{}{}
		ps.orderedVotes = append(ps.orderedVotes, vote)
		return true
	}
	return false
}

// All returns list of previously added votes in the same order they were added
func (ps *PendingVotes) All() []*model.Vote {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	votes := make([]*model.Vote, 0, len(ps.orderedVotes))
	for _, vote := range ps.orderedVotes {
		votes = append(votes, vote)
	}
	return votes
}

// NewPendingVotes creates a PendingVotes instance
func NewPendingVotes() *PendingVotes {
	return &PendingVotes{voteMap: make(map[flow.Identifier]struct{})}
}
