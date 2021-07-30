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

func (c *CollectionClusterVoteCollector) AddVote(vote *model.Vote) error {
	panic("implement me")
}

func (c *CollectionClusterVoteCollector) ProcessingStatus() hotstuff.ProcessingStatus {
	return hotstuff.VerifyingVotes
}

func (c *CollectionClusterVoteCollector) ChangeProcessingStatus(expectedValue, newValue hotstuff.ProcessingStatus) error {
	panic("implement me")
}

func (c *CollectionClusterVoteCollector) VoteCreator() hotstuff.CreateVote {
	panic("not implemented")
}
