package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
)

type OnQCCreatedConsumer = func(qc *flow.QuorumCertificate)

// QCCreatedDistributor subscribes for qc created event from hotstuff and distributes it to subscribers
// Objects are concurrency safe.
// NOTE: it can be refactored to work without lock since usually we never subscribe after startup. Mostly
// list of observers is static.
type QCCreatedDistributor struct {
	qcCreatedConsumers []OnQCCreatedConsumer
	lock               sync.RWMutex
}

var _ hotstuff.QCCreatedConsumer = (*QCCreatedDistributor)(nil)

func NewQCCreatedDistributor() *QCCreatedDistributor {
	return &QCCreatedDistributor{
		qcCreatedConsumers: make([]OnQCCreatedConsumer, 0),
	}
}

func (d *QCCreatedDistributor) AddConsumer(consumer OnQCCreatedConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.qcCreatedConsumers = append(d.qcCreatedConsumers, consumer)
}

func (d *QCCreatedDistributor) OnQcConstructedFromVotes(qc *flow.QuorumCertificate) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, consumer := range d.qcCreatedConsumers {
		consumer(qc)
	}
}
