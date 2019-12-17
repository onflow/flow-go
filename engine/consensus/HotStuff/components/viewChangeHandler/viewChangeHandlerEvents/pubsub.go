package viewChangeHandlerEvents

import (
	"reflect"
	"sync"

	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/def"
)

// PubSubEventProcessor implements viewChanger.Processor
// It allows thread-safe subscription to events
type PubSubEventProcessor struct {
	sentViewChangeConsumers     []SentViewChangeConsumer
	receivedViewChangeConsumers []ReceivedViewChangeConsumer
	lock                        sync.RWMutex
}

func New() *PubSubEventProcessor {
	return &PubSubEventProcessor{}
}

// OnSentViewChange calls OnSentViewChange on all consumer in `p.sentViewChangeConsumers`
func (p *PubSubEventProcessor) OnSentViewChange(qc *def.QuorumCertificate) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.sentViewChangeConsumers {
		subscriber.OnSentViewChange(qc)
	}
}

// AddSentViewChangeConsumer adds a SentViewChangeConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddSentViewChangeConsumer(cons SentViewChangeConsumer) *PubSubEventProcessor {
	ensureNotNil(cons)
	p.lock.Lock()
	defer p.lock.Unlock()
	p.sentViewChangeConsumers = append(p.sentViewChangeConsumers, cons)
	return p
}

// OnReceivedViewChange calls OnReceivedViewChange on all consumer in `p.sentViewChangeConsumers`
func (p *PubSubEventProcessor) OnReceivedViewChange(qc *def.QuorumCertificate) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.receivedViewChangeConsumers {
		subscriber.OnReceivedViewChange(qc)
	}
}

// AddReceivedViewChangeConsumer adds a ReceivedViewChangeConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddReceivedViewChangeConsumer(cons ReceivedViewChangeConsumer) *PubSubEventProcessor {
	ensureNotNil(cons)
	p.lock.Lock()
	defer p.lock.Unlock()
	p.receivedViewChangeConsumers = append(p.receivedViewChangeConsumers, cons)
	return p
}

func ensureNotNil(proc interface{}) {
	if proc == nil || reflect.ValueOf(proc).IsNil() {
		panic("Consumer cannot be nil")
	}
}
