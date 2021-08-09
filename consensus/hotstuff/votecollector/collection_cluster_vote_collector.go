package votecollector

import (
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

type CollectionClusterVoteCollector struct {
	BaseVoteCollector

	onQCCreated hotstuff.OnQCCreated
	done        atomic.Bool
}

// CreateVote implements BlockSigner interface for creating votes from block proposals
func (c *CollectionClusterVoteCollector) CreateVote(block *model.Block) (*model.Vote, error) {
	panic("implement me")
}

func (c *CollectionClusterVoteCollector) AddVote(vote *model.Vote) error {
	panic("implement me")
}

func (c *CollectionClusterVoteCollector) Status() hotstuff.VoteCollectorStatus {
	return hotstuff.VoteCollectorStatusVerifying
}

func NewCollectionClusterVoteCollector(base BaseVoteCollector) *CollectionClusterVoteCollector {
	return &CollectionClusterVoteCollector{
		BaseVoteCollector: base,
	}
}
