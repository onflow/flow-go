package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// VoteCollectorDistributor ingests events about QC creation from hotstuff and distributes them to subscribers.
// Objects are concurrency safe.
// NOTE: it can be refactored to work without lock since usually we never subscribe after startup. Mostly
// list of observers is static.
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
