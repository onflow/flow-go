package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// ParticipantDistributor ingests events from HotStuff's core logic and distributes them to
// consumers. This logic only runs inside active consensus participants proposing blocks, voting,
// collecting + aggregating votes to QCs, and participating in the pacemaker (sending timeouts,
// collecting + aggregating timeouts to TCs).
// Concurrently safe.
type ParticipantDistributor struct {
	consumers []hotstuff.ParticipantConsumer
	lock      sync.RWMutex
}

var _ hotstuff.ParticipantConsumer = (*ParticipantDistributor)(nil)

func NewParticipantDistributor() *ParticipantDistributor {
	return &ParticipantDistributor{}
}

func (d *ParticipantDistributor) AddParticipantConsumer(consumer hotstuff.ParticipantConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.consumers = append(d.consumers, consumer)
}

func (d *ParticipantDistributor) OnEventProcessed() {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnEventProcessed()
	}
}

func (d *ParticipantDistributor) OnStart(currentView uint64) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnStart(currentView)
	}
}

func (d *ParticipantDistributor) OnReceiveProposal(currentView uint64, proposal *model.Proposal) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnReceiveProposal(currentView, proposal)
	}
}

func (d *ParticipantDistributor) OnReceiveQc(currentView uint64, qc *flow.QuorumCertificate) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnReceiveQc(currentView, qc)
	}
}

func (d *ParticipantDistributor) OnReceiveTc(currentView uint64, tc *flow.TimeoutCertificate) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnReceiveTc(currentView, tc)
	}
}

func (d *ParticipantDistributor) OnPartialTc(currentView uint64, partialTc *hotstuff.PartialTcCreated) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnPartialTc(currentView, partialTc)
	}
}

func (d *ParticipantDistributor) OnLocalTimeout(currentView uint64) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnLocalTimeout(currentView)
	}
}

func (d *ParticipantDistributor) OnViewChange(oldView, newView uint64) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnViewChange(oldView, newView)
	}
}

func (d *ParticipantDistributor) OnQcTriggeredViewChange(oldView uint64, newView uint64, qc *flow.QuorumCertificate) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnQcTriggeredViewChange(oldView, newView, qc)
	}
}

func (d *ParticipantDistributor) OnTcTriggeredViewChange(oldView uint64, newView uint64, tc *flow.TimeoutCertificate) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnTcTriggeredViewChange(oldView, newView, tc)
	}
}

func (d *ParticipantDistributor) OnStartingTimeout(timerInfo model.TimerInfo) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnStartingTimeout(timerInfo)
	}
}

func (d *ParticipantDistributor) OnCurrentViewDetails(currentView, finalizedView uint64, currentLeader flow.Identifier) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnCurrentViewDetails(currentView, finalizedView, currentLeader)
	}
}
