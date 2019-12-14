package blockRequestEvents

import (
	"reflect"
	"sync"
)

// PubSubEventProcessor implements blockRequester.Processor
// It allows thread-safe subscription to events
type PubSubEventProcessor struct {
	blockRequestSentConsumers []BlockRequestSentConsumer
	lock                      sync.RWMutex
}

func New() *PubSubEventProcessor {
	return &PubSubEventProcessor{}
}

func (p *PubSubEventProcessor) OnBlockRequestSent(blockMRH []byte) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.blockRequestSentConsumers {
		subscriber.OnBlockRequestSent(blockMRH)
	}
}

// AddBlockRequestSentConsumer adds a BlockRequestSentConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddBlockRequestSentConsumer(cons BlockRequestSentConsumer) *PubSubEventProcessor {
	ensureNotNil(cons)
	p.lock.Lock()
	defer p.lock.Unlock()
	p.blockRequestSentConsumers = append(p.blockRequestSentConsumers, cons)
	return p
}

func ensureNotNil(proc interface{}) {
	if proc == nil || reflect.ValueOf(proc).IsNil() {
		panic("Consumer cannot be nil")
	}
}
