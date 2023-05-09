package pubsub

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
)

// Distributor distributes notifications to a list of subscribers (event consumers).
//
// It allows thread-safe subscription of multiple consumers to events.
type Distributor struct {
	*FollowerDistributor
	*CommunicatorDistributor
	*ParticipantDistributor
}

var _ hotstuff.Consumer = (*Distributor)(nil)

func NewDistributor() *Distributor {
	return &Distributor{
		FollowerDistributor:     NewFollowerDistributor(),
		CommunicatorDistributor: NewCommunicatorDistributor(),
		ParticipantDistributor:  NewParticipantDistributor(),
	}
}

// AddConsumer adds an event consumer to the Distributor
func (p *Distributor) AddConsumer(consumer hotstuff.Consumer) {
	p.FollowerDistributor.AddFollowerConsumer(consumer)
	p.CommunicatorDistributor.AddCommunicatorConsumer(consumer)
	p.ParticipantDistributor.AddParticipantConsumer(consumer)
}

// FollowerDistributor ingests consensus follower events and distributes it to subscribers.
type FollowerDistributor struct {
	*ProtocolViolationDistributor
	*FinalizationDistributor
}

var _ hotstuff.FollowerConsumer = (*FollowerDistributor)(nil)

func NewFollowerDistributor() *FollowerDistributor {
	return &FollowerDistributor{
		ProtocolViolationDistributor: NewProtocolViolationDistributor(),
		FinalizationDistributor:      NewFinalizationDistributor(),
	}
}

func (d *FollowerDistributor) AddFollowerConsumer(consumer hotstuff.FollowerConsumer) {
	d.FinalizationDistributor.AddFinalizationConsumer(consumer)
	d.ProtocolViolationDistributor.AddProtocolViolationConsumer(consumer)
}

type TimeoutAggregationDistributor struct {
	*TimeoutAggregationViolationDistributor
	*TimeoutCollectorDistributor
}

var _ hotstuff.TimeoutAggregationConsumer = (*TimeoutAggregationDistributor)(nil)

func NewTimeoutAggregationDistributor() *TimeoutAggregationDistributor {
	return &TimeoutAggregationDistributor{
		TimeoutAggregationViolationDistributor: NewTimeoutAggregationViolationDistributor(),
		TimeoutCollectorDistributor:            NewTimeoutCollectorDistributor(),
	}
}

func (d *TimeoutAggregationDistributor) AddTimeoutAggregationConsumer(consumer hotstuff.TimeoutAggregationConsumer) {
	d.TimeoutAggregationViolationDistributor.AddTimeoutAggregationViolationConsumer(consumer)
	d.TimeoutCollectorDistributor.AddTimeoutCollectorConsumer(consumer)
}

type VoteAggregationDistributor struct {
	*VoteAggregationViolationDistributor
	*QCCreatedDistributor
}

var _ hotstuff.VoteAggregationConsumer = (*VoteAggregationDistributor)(nil)

func NewVoteAggregationDistributor() *VoteAggregationDistributor {
	return &VoteAggregationDistributor{
		VoteAggregationViolationDistributor: NewVoteAggregationViolationDistributor(),
		QCCreatedDistributor:                NewQCCreatedDistributor(),
	}
}

func (d *VoteAggregationDistributor) AddVoteAggregationConsumer(consumer hotstuff.VoteAggregationConsumer) {
	d.VoteAggregationViolationDistributor.AddVoteAggregationViolationConsumer(consumer)
	d.QCCreatedDistributor.AddQCCreatedConsumer(consumer)
}
