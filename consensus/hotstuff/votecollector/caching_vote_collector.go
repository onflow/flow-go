package votecollector

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
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

	// TODO: at this point we don't have any way to ensure vote validity since we don't
	// check signature, we can only collect votes and make decisions about double votes
	// and protocol violations once we check signature. In current version it has to happen after
	// we have received a block proposal. It's a potential optimization to check signature as soon as we
	// receive the vote but it requires knowledge about identity table which might not be up to date
	// without actually receiving proposal.

	_ = c.pendingVotes.AddVote(vote)

	return nil
}

func (c *CachingVoteCollector) Status() hotstuff.VoteCollectorStatus {
	return hotstuff.VoteCollectorStatusCaching
}

func (c *CachingVoteCollector) GetVotesByBlockID(blockID flow.Identifier) []*model.Vote {
	return c.pendingVotes.ByBlockID(blockID)
}
