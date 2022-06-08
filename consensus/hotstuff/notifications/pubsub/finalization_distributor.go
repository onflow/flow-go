package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

type OnBlockFinalizedConsumer = func(block *model.Block)
type OnBlockIncorporatedConsumer = func(block *model.Block)

// FinalizationDistributor subscribes for finalization events from hotstuff and distributes it to subscribers
type FinalizationDistributor struct {
	blockFinalizedConsumers       []OnBlockFinalizedConsumer
	blockIncorporatedConsumers    []OnBlockIncorporatedConsumer
	hotStuffFinalizationConsumers []hotstuff.FinalizationConsumer
	lock                          sync.RWMutex
}

var _ hotstuff.Consumer = (*FinalizationDistributor)(nil)

func NewFinalizationDistributor() *FinalizationDistributor {
	return &FinalizationDistributor{
		blockFinalizedConsumers:    make([]OnBlockFinalizedConsumer, 0),
		blockIncorporatedConsumers: make([]OnBlockIncorporatedConsumer, 0),
		lock:                       sync.RWMutex{},
	}
}

func (p *FinalizationDistributor) AddOnBlockFinalizedConsumer(consumer OnBlockFinalizedConsumer) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.blockFinalizedConsumers = append(p.blockFinalizedConsumers, consumer)
}

func (p *FinalizationDistributor) AddOnBlockIncorporatedConsumer(consumer OnBlockIncorporatedConsumer) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.blockIncorporatedConsumers = append(p.blockIncorporatedConsumers, consumer)
}

func (p *FinalizationDistributor) AddConsumer(consumer hotstuff.FinalizationConsumer) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.hotStuffFinalizationConsumers = append(p.hotStuffFinalizationConsumers, consumer)
}

func (p *FinalizationDistributor) OnEventProcessed() {}

func (p *FinalizationDistributor) OnBlockIncorporated(block *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, consumer := range p.blockIncorporatedConsumers {
		consumer(block)
	}
	for _, consumer := range p.hotStuffFinalizationConsumers {
		consumer.OnBlockIncorporated(block)
	}
}

func (p *FinalizationDistributor) OnFinalizedBlock(block *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, consumer := range p.blockFinalizedConsumers {
		consumer(block)
	}
	for _, consumer := range p.hotStuffFinalizationConsumers {
		consumer.OnFinalizedBlock(block)
	}
}

func (p *FinalizationDistributor) OnDoubleProposeDetected(block1, block2 *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, consumer := range p.hotStuffFinalizationConsumers {
		consumer.OnDoubleProposeDetected(block1, block2)
	}
}

func (p *FinalizationDistributor) OnReceiveVote(uint64, *model.Vote) {}

func (p *FinalizationDistributor) OnReceiveProposal(uint64, *model.Proposal) {}

func (p *FinalizationDistributor) OnEnteringView(uint64, flow.Identifier) {}

func (p *FinalizationDistributor) OnQcTriggeredViewChange(*flow.QuorumCertificate, uint64) {}

func (p *FinalizationDistributor) OnProposingBlock(*model.Proposal) {}

func (p *FinalizationDistributor) OnVoting(*model.Vote) {}

func (p *FinalizationDistributor) OnQcConstructedFromVotes(curView uint64, qc *flow.QuorumCertificate) {
}

func (p *FinalizationDistributor) OnStartingTimeout(*model.TimerInfo) {}

func (p *FinalizationDistributor) OnReachedTimeout(*model.TimerInfo) {}

func (p *FinalizationDistributor) OnQcIncorporated(*flow.QuorumCertificate) {}

func (p *FinalizationDistributor) OnForkChoiceGenerated(uint64, *flow.QuorumCertificate) {}

func (p *FinalizationDistributor) OnDoubleVotingDetected(*model.Vote, *model.Vote) {}

func (p *FinalizationDistributor) OnInvalidVoteDetected(*model.Vote) {}

func (p *FinalizationDistributor) OnVoteForInvalidBlockDetected(*model.Vote, *model.Proposal) {}
