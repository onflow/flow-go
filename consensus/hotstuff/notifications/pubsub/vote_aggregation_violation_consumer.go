package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// VoteAggregationViolationDistributor ingests notifications about vote aggregation violations and
// distributes them to consumers. Such notifications are produced by the vote aggregation logic.
// Concurrently safe.
type VoteAggregationViolationDistributor struct {
	consumers []hotstuff.VoteAggregationViolationConsumer
	lock      sync.RWMutex
}

var _ hotstuff.VoteAggregationViolationConsumer = (*VoteAggregationViolationDistributor)(nil)

func NewVoteAggregationViolationDistributor() *VoteAggregationViolationDistributor {
	return &VoteAggregationViolationDistributor{}
}

func (d *VoteAggregationViolationDistributor) AddVoteAggregationViolationConsumer(consumer hotstuff.VoteAggregationViolationConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.consumers = append(d.consumers, consumer)
}

func (d *VoteAggregationViolationDistributor) OnDoubleVotingDetected(vote1, vote2 *model.Vote) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnDoubleVotingDetected(vote1, vote2)
	}
}

func (d *VoteAggregationViolationDistributor) OnInvalidVoteDetected(err model.InvalidVoteError) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnInvalidVoteDetected(err)
	}
}

func (d *VoteAggregationViolationDistributor) OnVoteForInvalidBlockDetected(vote *model.Vote, invalidProposal *model.Proposal) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnVoteForInvalidBlockDetected(vote, invalidProposal)
	}
}
