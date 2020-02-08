package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

// PendingStatus keeps track of pending votes for the same block
type PendingStatus struct {
	// When receiving missing block, first received votes will be accumulated
	orderedVotes []*types.Vote
	// For avoiding duplicate votes
	voteMap map[string]*types.Vote
}

func (ps *PendingStatus) AddVote(vote *types.Vote) {
	_, exists := ps.voteMap[vote.ID().String()]
	if exists {
		return
	}
	ps.voteMap[vote.ID().String()] = vote
	ps.orderedVotes = append(ps.orderedVotes, vote)
}

func NewPendingStatus() *PendingStatus {
	return &PendingStatus{
		voteMap: map[string]*types.Vote{},
	}
}
