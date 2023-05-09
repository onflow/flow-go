package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// ProtocolViolationDistributor ingests notifications about HotStuff-protocol violations and
// distributes them to subscribers. Such notifications are produced by the active consensus
// participants and to a lesser degree also the consensus follower.
// Concurrently safe.
type ProtocolViolationDistributor struct {
	subscribers []hotstuff.ProposalViolationConsumer
	lock        sync.RWMutex
}

var _ hotstuff.ProposalViolationConsumer = (*ProtocolViolationDistributor)(nil)

func NewProtocolViolationDistributor() *ProtocolViolationDistributor {
	return &ProtocolViolationDistributor{}
}

func (d *ProtocolViolationDistributor) AddProtocolViolationConsumer(consumer hotstuff.ProposalViolationConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.subscribers = append(d.subscribers, consumer)
}

func (d *ProtocolViolationDistributor) OnInvalidBlockDetected(err model.InvalidBlockError) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.subscribers {
		subscriber.OnInvalidBlockDetected(err)
	}
}

func (d *ProtocolViolationDistributor) OnDoubleProposeDetected(block1, block2 *model.Block) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.subscribers {
		subscriber.OnDoubleProposeDetected(block1, block2)
	}
}
