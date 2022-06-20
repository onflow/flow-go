package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
)

// TimeoutCollectorDistributor subscribes for events from hotstuff and distributes it to subscribers.
// Objects are concurrency safe.
type TimeoutCollectorDistributor struct {
	consumers []hotstuff.TimeoutCollectorConsumer
	lock      sync.RWMutex
}

var _ hotstuff.TimeoutCollectorConsumer = (*TimeoutCollectorDistributor)(nil)

func NewTimeoutCollectorDistributor() *TimeoutCollectorDistributor {
	return &TimeoutCollectorDistributor{
		consumers: make([]hotstuff.TimeoutCollectorConsumer, 0),
	}
}

func (d *TimeoutCollectorDistributor) AddConsumer(consumer hotstuff.TimeoutCollectorConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.consumers = append(d.consumers, consumer)
}

func (d *TimeoutCollectorDistributor) OnTcConstructedFromTimeouts(tc *flow.TimeoutCertificate) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, consumer := range d.consumers {
		consumer.OnTcConstructedFromTimeouts(tc)
	}
}

func (d *TimeoutCollectorDistributor) OnPartialTcCreated(view uint64) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, consumer := range d.consumers {
		consumer.OnPartialTcCreated(view)
	}
}

func (d *TimeoutCollectorDistributor) OnNewQcDiscovered(qc *flow.QuorumCertificate) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, consumer := range d.consumers {
		consumer.OnNewQcDiscovered(qc)
	}
}

func (d *TimeoutCollectorDistributor) OnNewTcDiscovered(tc *flow.TimeoutCertificate) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, consumer := range d.consumers {
		consumer.OnNewTcDiscovered(tc)
	}
}
