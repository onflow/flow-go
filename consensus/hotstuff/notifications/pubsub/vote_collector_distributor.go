package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// VoteCollectorDistributor ingests notifications about vote aggregation and
// distributes them to consumers. Such notifications are produced by the vote aggregation logic.
// Concurrently safe.
type VoteCollectorDistributor struct {
	consumers []hotstuff.VoteCollectorConsumer
	lock      sync.RWMutex
}

var _ hotstuff.VoteCollectorConsumer = (*VoteCollectorDistributor)(nil)

func NewQCCreatedDistributor() *VoteCollectorDistributor {
	return &VoteCollectorDistributor{}
}

func (d *VoteCollectorDistributor) AddVoteCollectorConsumer(consumer hotstuff.VoteCollectorConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.consumers = append(d.consumers, consumer)
}

func (d *VoteCollectorDistributor) OnQcConstructedFromVotes(qc *flow.QuorumCertificate) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, consumer := range d.consumers {
		consumer.OnQcConstructedFromVotes(qc)
	}
}

func (d *VoteCollectorDistributor) OnVoteProcessed(vote *model.Vote) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnVoteProcessed(vote)
	}
}
