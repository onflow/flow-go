package events

import (
	"sync"

	"github.com/dapperlabs/flow-go/engine/consensus/eventdriven/modules/def"
	"github.com/dapperlabs/flow-go/engine/consensus/eventdriven/modules/utils"
)

// PubSubEventProcessor implements core.Processor
// It allows thread-safe subscription to events
type PubSubEventProcessor struct {
	incorporatedQcCons []IncorporatedQuorumCertificateConsumer
	forkChoiceCons     []ForkChoiceGeneratedConsumer
	lock               sync.RWMutex
}

func NewPubSubEventProcessor() *PubSubEventProcessor {
	return &PubSubEventProcessor{}
}

func (p *PubSubEventProcessor) OnQcFromVotesIncorporated(qc *def.QuorumCertificate) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.incorporatedQcCons {
		subscriber.OnQcFromVotesIncorporated(qc)
	}
}

func (p *PubSubEventProcessor) OnForkChoiceGenerated(viewNumber uint64, qc *def.QuorumCertificate) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.forkChoiceCons {
		subscriber.OnForkChoiceGenerated(viewNumber, qc)
	}
}

// AddIncorporatedQuorumCertificateConsumer adds an IncorporatedQuorumCertificateConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddIncorporatedQuorumCertificateConsumer(cons IncorporatedQuorumCertificateConsumer) *PubSubEventProcessor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.incorporatedQcCons = append(p.incorporatedQcCons, cons)
	return p
}

// AddForkChoiceConsumer adds a ForkChoiceConsumer to the PubSubEventProcessor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubEventProcessor) AddForkChoiceConsumer(cons ForkChoiceGeneratedConsumer) *PubSubEventProcessor {
	utils.EnsureNotNil(cons, "Event consumer")
	p.lock.Lock()
	defer p.lock.Unlock()
	p.forkChoiceCons = append(p.forkChoiceCons, cons)
	return p
}
