package votecollector

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

type CachingVoteCollector struct {
	CollectionBase
	pendingVotes *PendingVotes
}

func NewCachingVoteCollector(base CollectionBase) *CachingVoteCollector {
	return &CachingVoteCollector{
		CollectionBase: base,
		pendingVotes:   NewPendingVotes(),
	}
}

func (c *CachingVoteCollector) AddVote(vote *model.Vote) error {
	if vote.View != c.view {
		return fmt.Errorf("this CachingVoteCollector processes votes for view %d, "+
			"but got a vote for %d", c.view, vote.View)
	}

	// TODO: add handling of double votes here. If we don't know anything about block
	// proposal we might delay our decision, but still double votes has to be tracked.
	// Use c.doubleVoteDetector for this purpose.

	_ = c.pendingVotes.AddVote(vote)

	return nil
}

func (c *CachingVoteCollector) Status() hotstuff.VoteCollectorStatus {
	return hotstuff.VoteCollectorStatusCaching
}

func (c *CachingVoteCollector) GetVotesByBlockID(blockID flow.Identifier) []*model.Vote {
	return c.pendingVotes.ByBlockID(blockID)
}
