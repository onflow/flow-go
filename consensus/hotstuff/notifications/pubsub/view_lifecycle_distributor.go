package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// ViewLifecycleDistributor ingests events from HotStuff's core logic and distributes them to
// consumers. This logic only runs inside active consensus participants proposing blocks, voting,
// collecting + aggregating votes to QCs, and participating in the pacemaker (sending timeouts,
// collecting + aggregating timeouts to TCs).
// Concurrently safe.
type ViewLifecycleDistributor struct {
	consumers []hotstuff.ViewLifecycleConsumer
	lock      sync.RWMutex
}

var _ hotstuff.ViewLifecycleConsumer = (*ViewLifecycleDistributor)(nil)

func NewViewLifecycleDistributor() *ViewLifecycleDistributor {
	return &ViewLifecycleDistributor{}
}

func (d *ViewLifecycleDistributor) AddViewLifecycleConsumer(consumer hotstuff.ViewLifecycleConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.consumers = append(d.consumers, consumer)
}

func (d *ViewLifecycleDistributor) OnEventProcessed() {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnEventProcessed()
	}
}

func (d *ViewLifecycleDistributor) OnStart(currentView uint64) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnStart(currentView)
	}
}

func (d *ViewLifecycleDistributor) OnReceiveProposal(currentView uint64, proposal *model.Proposal) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnReceiveProposal(currentView, proposal)
	}
}

func (d *ViewLifecycleDistributor) OnReceiveQc(currentView uint64, qc *flow.QuorumCertificate) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnReceiveQc(currentView, qc)
	}
}

func (d *ViewLifecycleDistributor) OnReceiveTc(currentView uint64, tc *flow.TimeoutCertificate) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnReceiveTc(currentView, tc)
	}
}

func (d *ViewLifecycleDistributor) OnPartialTc(currentView uint64, partialTc *hotstuff.PartialTcCreated) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnPartialTc(currentView, partialTc)
	}
}

func (d *ViewLifecycleDistributor) OnLocalTimeout(currentView uint64) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnLocalTimeout(currentView)
	}
}

func (d *ViewLifecycleDistributor) OnViewChange(oldView, newView uint64) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnViewChange(oldView, newView)
	}
}

func (d *ViewLifecycleDistributor) OnQcTriggeredViewChange(oldView uint64, newView uint64, qc *flow.QuorumCertificate) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnQcTriggeredViewChange(oldView, newView, qc)
	}
}

func (d *ViewLifecycleDistributor) OnTcTriggeredViewChange(oldView uint64, newView uint64, tc *flow.TimeoutCertificate) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnTcTriggeredViewChange(oldView, newView, tc)
	}
}

func (d *ViewLifecycleDistributor) OnStartingTimeout(timerInfo model.TimerInfo) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnStartingTimeout(timerInfo)
	}
}

func (d *ViewLifecycleDistributor) OnCurrentViewDetails(currentView, finalizedView uint64, currentLeader flow.Identifier) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnCurrentViewDetails(currentView, finalizedView, currentLeader)
	}
}
