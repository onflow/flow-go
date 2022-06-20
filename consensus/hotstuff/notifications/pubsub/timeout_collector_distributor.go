package pubsub

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
)

// timeoutCollectorDistributor subscribes for events from hotstuff and distributes it to subscribers.
// timeoutCollectorDistributor is created with fixed set of signers using Builder pattern.
// Once we create an instance consumers never change.
type timeoutCollectorDistributor struct {
	consumers []hotstuff.TimeoutCollectorConsumer
}

// TimeoutCollectorDistributorBuilder is an exportable type which is used to build the distributor.
// We use this structure to safely add consumers and create a distributor which won't be modified after creation.
// This approach is needed to avoid introducing lock for consumers as we don't have guarantees that it won't be modified.
type TimeoutCollectorDistributorBuilder struct {
	consumers []hotstuff.TimeoutCollectorConsumer
}

var _ hotstuff.TimeoutCollectorConsumer = (*timeoutCollectorDistributor)(nil)

func NewTimeoutCollectorDistributorBuilder() *TimeoutCollectorDistributorBuilder {
	return &TimeoutCollectorDistributorBuilder{
		consumers: make([]hotstuff.TimeoutCollectorConsumer, 0),
	}
}

func (d *TimeoutCollectorDistributorBuilder) AddConsumer(consumer hotstuff.TimeoutCollectorConsumer) {
	d.consumers = append(d.consumers, consumer)
}

func (d *TimeoutCollectorDistributorBuilder) Build() hotstuff.TimeoutCollectorConsumer {
	return &timeoutCollectorDistributor{consumers: d.consumers}
}

func (d *timeoutCollectorDistributor) OnTcConstructedFromTimeouts(tc *flow.TimeoutCertificate) {
	for _, consumer := range d.consumers {
		consumer.OnTcConstructedFromTimeouts(tc)
	}
}

func (d *timeoutCollectorDistributor) OnPartialTcCreated(view uint64) {
	for _, consumer := range d.consumers {
		consumer.OnPartialTcCreated(view)
	}
}

func (d *timeoutCollectorDistributor) OnNewQcDiscovered(qc *flow.QuorumCertificate) {
	for _, consumer := range d.consumers {
		consumer.OnNewQcDiscovered(qc)
	}
}

func (d *timeoutCollectorDistributor) OnNewTcDiscovered(tc *flow.TimeoutCertificate) {
	for _, consumer := range d.consumers {
		consumer.OnNewTcDiscovered(tc)
	}
}
