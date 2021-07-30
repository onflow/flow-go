package votecollector

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

type CachingVoteCollector struct {
	BaseVoteCollector
	pendingVotes *PendingVotes
}

func NewCachingVoteCollector(base BaseVoteCollector) *CachingVoteCollector {
	return &CachingVoteCollector{
		BaseVoteCollector: base,
		pendingVotes:      NewPendingVotes(),
	}
}

func (c *CachingVoteCollector) AddVote(vote *model.Vote) error {
	if vote.BlockID != c.blockID {
		return fmt.Errorf("this CachingVoteCollector processes votes for blockID (%x), "+
			"but got a vote for (%x)", c.blockID, vote.BlockID)
	}

	_ = c.pendingVotes.AddVote(vote)

	return nil
}

func (c *CachingVoteCollector) VoteCreator() hotstuff.CreateVote {
	panic("not implemented")
}

func (c *CachingVoteCollector) ProcessingStatus() hotstuff.ProcessingStatus {
	return hotstuff.CachingVotes
}

func (c *CachingVoteCollector) GetVotes() []*model.Vote {
	return c.pendingVotes.All()
}
