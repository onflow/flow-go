package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
)

// TimeoutCollectorDistributor subscribes for events from hotstuff and distributes it to subscribers.
// Concurrently safe
// TODO: investigate if this can be updated using atomics to prevent locking on mutex since we always add all consumers
// before delivering events.
type TimeoutCollectorDistributor struct {
	lock      sync.Mutex
	consumers []hotstuff.TimeoutCollectorConsumer
}

var _ hotstuff.TimeoutCollectorConsumer = (*TimeoutCollectorDistributor)(nil)

func (d *TimeoutCollectorDistributor) AddConsumer(consumer hotstuff.TimeoutCollectorConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.consumers = append(d.consumers, consumer)
}

func (d *TimeoutCollectorDistributor) OnTcConstructedFromTimeouts(tc *flow.TimeoutCertificate) {
	for _, consumer := range d.consumers {
		consumer.OnTcConstructedFromTimeouts(tc)
	}
}

func (d *TimeoutCollectorDistributor) OnPartialTcCreated(view uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) {
	for _, consumer := range d.consumers {
		consumer.OnPartialTcCreated(view, newestQC, lastViewTC)
	}
}

func (d *TimeoutCollectorDistributor) OnNewQcDiscovered(qc *flow.QuorumCertificate) {

	for _, consumer := range d.consumers {
		consumer.OnNewQcDiscovered(qc)
	}
}

func (d *TimeoutCollectorDistributor) OnNewTcDiscovered(tc *flow.TimeoutCertificate) {
	for _, consumer := range d.consumers {
		consumer.OnNewTcDiscovered(tc)
	}
}
