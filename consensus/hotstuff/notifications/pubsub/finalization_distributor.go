package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

type FinalizationConsumer = func(finalizedBlockID flow.Identifier)

// FinalizationDistributor subscribes for finalization events from hotstuff and distributes it to subscribers
type FinalizationDistributor struct {
	subscribers []FinalizationConsumer
	lock        sync.RWMutex
}

func NewFinalizationDistributor() *FinalizationDistributor {
	return &FinalizationDistributor{
		subscribers: make([]FinalizationConsumer, 0),
		lock:        sync.RWMutex{},
	}
}

func (p *FinalizationDistributor) AddConsumer(consumer FinalizationConsumer) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.subscribers = append(p.subscribers, consumer)
}

func (p *FinalizationDistributor) OnEventProcessed() {}

func (p *FinalizationDistributor) OnBlockIncorporated(*model.Block) {}

func (p *FinalizationDistributor) OnFinalizedBlock(block *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, consumer := range p.subscribers {
		consumer(block.BlockID)
	}
}

func (p *FinalizationDistributor) OnDoubleProposeDetected(*model.Block, *model.Block) {}

func (p *FinalizationDistributor) OnReceiveVote(uint64, *model.Vote) {}

func (p *FinalizationDistributor) OnReceiveProposal(uint64, *model.Proposal) {}

func (p *FinalizationDistributor) OnEnteringView(uint64, flow.Identifier) {}

func (p *FinalizationDistributor) OnQcTriggeredViewChange(*flow.QuorumCertificate, uint64) {}

func (p *FinalizationDistributor) OnProposingBlock(*model.Proposal) {}

func (p *FinalizationDistributor) OnVoting(*model.Vote) {}

func (p *FinalizationDistributor) OnQcConstructedFromVotes(*flow.QuorumCertificate) {}

func (p *FinalizationDistributor) OnStartingTimeout(*model.TimerInfo) {}

func (p *FinalizationDistributor) OnReachedTimeout(*model.TimerInfo) {}

func (p *FinalizationDistributor) OnQcIncorporated(*flow.QuorumCertificate) {}

func (p *FinalizationDistributor) OnForkChoiceGenerated(uint64, *flow.QuorumCertificate) {}

func (p *FinalizationDistributor) OnDoubleVotingDetected(*model.Vote, *model.Vote) {}

func (p *FinalizationDistributor) OnInvalidVoteDetected(*model.Vote) {}
