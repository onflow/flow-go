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
	subscribers []hotstuff.ProtocolViolationConsumer
	lock        sync.RWMutex
}

var _ hotstuff.ProtocolViolationConsumer = (*ProtocolViolationDistributor)(nil)

func NewProtocolViolationDistributor() *ProtocolViolationDistributor {
	return &ProtocolViolationDistributor{}
}

func (d *ProtocolViolationDistributor) AddProtocolViolationConsumer(consumer hotstuff.ProtocolViolationConsumer) {
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

func (d *ProtocolViolationDistributor) OnDoubleVotingDetected(vote1, vote2 *model.Vote) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.subscribers {
		subscriber.OnDoubleVotingDetected(vote1, vote2)
	}
}

func (d *ProtocolViolationDistributor) OnInvalidVoteDetected(err model.InvalidVoteError) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.subscribers {
		subscriber.OnInvalidVoteDetected(err)
	}
}

func (d *ProtocolViolationDistributor) OnVoteForInvalidBlockDetected(vote *model.Vote, invalidProposal *model.Proposal) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.subscribers {
		subscriber.OnVoteForInvalidBlockDetected(vote, invalidProposal)
	}
}

func (d *ProtocolViolationDistributor) OnDoubleTimeoutDetected(timeout *model.TimeoutObject, altTimeout *model.TimeoutObject) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.subscribers {
		subscriber.OnDoubleTimeoutDetected(timeout, altTimeout)
	}
}

func (d *ProtocolViolationDistributor) OnInvalidTimeoutDetected(err model.InvalidTimeoutError) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.subscribers {
		subscriber.OnInvalidTimeoutDetected(err)
	}
}
