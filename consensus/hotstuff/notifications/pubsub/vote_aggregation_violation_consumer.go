package pubsub

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"sync"
)

// VoteAggregationViolationDistributor ingests notifications about HotStuff-protocol violations and
// distributes them to subscribers. Such notifications are produced by the active consensus
// participants and to a lesser degree also the consensus follower.
// Concurrently safe.
type VoteAggregationViolationDistributor struct {
	subscribers []hotstuff.VoteAggregationViolationConsumer
	lock        sync.RWMutex
}

var _ hotstuff.VoteAggregationViolationConsumer = (*VoteAggregationViolationDistributor)(nil)

func NewVoteAggregationViolationDistributor() *VoteAggregationViolationDistributor {
	return &VoteAggregationViolationDistributor{}
}

func (d *VoteAggregationViolationDistributor) AddVoteAggregationViolationConsumer(consumer hotstuff.VoteAggregationViolationConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.subscribers = append(d.subscribers, consumer)
}

func (d *VoteAggregationViolationDistributor) OnDoubleVotingDetected(vote1, vote2 *model.Vote) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.subscribers {
		subscriber.OnDoubleVotingDetected(vote1, vote2)
	}
}

func (d *VoteAggregationViolationDistributor) OnInvalidVoteDetected(err model.InvalidVoteError) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.subscribers {
		subscriber.OnInvalidVoteDetected(err)
	}
}

func (d *VoteAggregationViolationDistributor) OnVoteForInvalidBlockDetected(vote *model.Vote, invalidProposal *model.Proposal) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.subscribers {
		subscriber.OnVoteForInvalidBlockDetected(vote, invalidProposal)
	}
}
