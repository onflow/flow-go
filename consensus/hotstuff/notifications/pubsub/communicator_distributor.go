package pubsub

import (
	"sync"
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// CommunicatorDistributor ingests outbound consensus messages from HotStuff's core logic and
// distributes them to consumers. This logic only runs inside active consensus participants proposing
// blocks, voting, collecting + aggregating votes to QCs, and participating in the pacemaker (sending
// timeouts, collecting + aggregating timeouts to TCs).
// Concurrently safe.
type CommunicatorDistributor struct {
	consumers []hotstuff.CommunicatorConsumer
	lock      sync.RWMutex
}

var _ hotstuff.CommunicatorConsumer = (*CommunicatorDistributor)(nil)

func NewCommunicatorDistributor() *CommunicatorDistributor {
	return &CommunicatorDistributor{}
}

func (d *CommunicatorDistributor) AddCommunicatorConsumer(consumer hotstuff.CommunicatorConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.consumers = append(d.consumers, consumer)
}

func (d *CommunicatorDistributor) OnOwnVote(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, s := range d.consumers {
		s.OnOwnVote(blockID, view, sigData, recipientID)
	}
}

func (d *CommunicatorDistributor) OnOwnTimeout(timeout *model.TimeoutObject) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, s := range d.consumers {
		s.OnOwnTimeout(timeout)
	}
}

func (d *CommunicatorDistributor) OnOwnProposal(proposal *flow.Header, targetPublicationTime time.Time) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, s := range d.consumers {
		s.OnOwnProposal(proposal, targetPublicationTime)
	}
}
