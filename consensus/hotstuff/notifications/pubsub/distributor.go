package pubsub

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
)

// Distributor distributes notifications to a list of consumers (event consumers).
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

// FollowerDistributor ingests consensus follower events and distributes it to consumers.
// It allows thread-safe subscription of multiple consumers to events.
type FollowerDistributor struct {
	*ProposalViolationDistributor
	*FinalizationDistributor
}

var _ hotstuff.FollowerConsumer = (*FollowerDistributor)(nil)

func NewFollowerDistributor() *FollowerDistributor {
	return &FollowerDistributor{
		ProposalViolationDistributor: NewProtocolViolationDistributor(),
		FinalizationDistributor:      NewFinalizationDistributor(),
	}
}

// AddFollowerConsumer registers the input `consumer` to be notified on `hotstuff.ConsensusFollowerConsumer` events.
func (d *FollowerDistributor) AddFollowerConsumer(consumer hotstuff.FollowerConsumer) {
	d.FinalizationDistributor.AddFinalizationConsumer(consumer)
	d.ProposalViolationDistributor.AddProposalViolationConsumer(consumer)
}

// TimeoutAggregationDistributor ingests timeout aggregation events and distributes it to consumers.
// It allows thread-safe subscription of multiple consumers to events.
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

// VoteAggregationDistributor ingests vote aggregation events and distributes it to consumers.
// It allows thread-safe subscription of multiple consumers to events.
type VoteAggregationDistributor struct {
	*VoteAggregationViolationDistributor
	*VoteCollectorDistributor
}

var _ hotstuff.VoteAggregationConsumer = (*VoteAggregationDistributor)(nil)

func NewVoteAggregationDistributor() *VoteAggregationDistributor {
	return &VoteAggregationDistributor{
		VoteAggregationViolationDistributor: NewVoteAggregationViolationDistributor(),
		VoteCollectorDistributor:            NewQCCreatedDistributor(),
	}
}

func (d *VoteAggregationDistributor) AddVoteAggregationConsumer(consumer hotstuff.VoteAggregationConsumer) {
	d.VoteAggregationViolationDistributor.AddVoteAggregationViolationConsumer(consumer)
	d.VoteCollectorDistributor.AddVoteCollectorConsumer(consumer)
}
