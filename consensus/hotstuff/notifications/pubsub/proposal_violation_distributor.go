package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// ProposalViolationDistributor ingests notifications about HotStuff-protocol violations and
// distributes them to consumers. Such notifications are produced by the active consensus
// participants and the consensus follower.
// Concurrently safe.
type ProposalViolationDistributor struct {
	consumers []hotstuff.ProposalViolationConsumer
	lock      sync.RWMutex
}

var _ hotstuff.ProposalViolationConsumer = (*ProposalViolationDistributor)(nil)

func NewProtocolViolationDistributor() *ProposalViolationDistributor {
	return &ProposalViolationDistributor{}
}

func (d *ProposalViolationDistributor) AddProposalViolationConsumer(consumer hotstuff.ProposalViolationConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.consumers = append(d.consumers, consumer)
}

func (d *ProposalViolationDistributor) OnInvalidBlockDetected(err flow.Slashable[model.InvalidProposalError]) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnInvalidBlockDetected(err)
	}
}

func (d *ProposalViolationDistributor) OnDoubleProposeDetected(block1, block2 *model.Block) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnDoubleProposeDetected(block1, block2)
	}
}
