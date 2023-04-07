package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

type OnBlockFinalizedConsumer = func(block *model.Block)
type OnBlockIncorporatedConsumer = func(block *model.Block)

// FollowerDistributor ingests consensus follower events and distributes it to subscribers.
type FollowerDistributor struct {
	blockFinalizedConsumers    []OnBlockFinalizedConsumer
	blockIncorporatedConsumers []OnBlockIncorporatedConsumer
	followerConsumers          []hotstuff.ConsensusFollowerConsumer
	lock                       sync.RWMutex
}

var _ hotstuff.ConsensusFollowerConsumer = (*FollowerDistributor)(nil)

func NewFollowerDistributor() *FollowerDistributor {
	return &FollowerDistributor{
		blockFinalizedConsumers:    make([]OnBlockFinalizedConsumer, 0),
		blockIncorporatedConsumers: make([]OnBlockIncorporatedConsumer, 0),
		lock:                       sync.RWMutex{},
	}
}

func (p *FollowerDistributor) AddOnBlockFinalizedConsumer(consumer OnBlockFinalizedConsumer) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.blockFinalizedConsumers = append(p.blockFinalizedConsumers, consumer)
}

func (p *FollowerDistributor) AddOnBlockIncorporatedConsumer(consumer OnBlockIncorporatedConsumer) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.blockIncorporatedConsumers = append(p.blockIncorporatedConsumers, consumer)
}

func (p *FollowerDistributor) AddConsumer(consumer hotstuff.ConsensusFollowerConsumer) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.followerConsumers = append(p.followerConsumers, consumer)
}

func (p *FollowerDistributor) OnBlockIncorporated(block *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, consumer := range p.blockIncorporatedConsumers {
		consumer(block)
	}
	for _, consumer := range p.followerConsumers {
		consumer.OnBlockIncorporated(block)
	}
}

func (p *FollowerDistributor) OnFinalizedBlock(block *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, consumer := range p.blockFinalizedConsumers {
		consumer(block)
	}
	for _, consumer := range p.followerConsumers {
		consumer.OnFinalizedBlock(block)
	}
}

func (p *FollowerDistributor) OnDoubleProposeDetected(block1, block2 *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, consumer := range p.followerConsumers {
		consumer.OnDoubleProposeDetected(block1, block2)
	}
}

func (p *FollowerDistributor) OnInvalidBlockDetected(err model.InvalidBlockError) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, consumer := range p.followerConsumers {
		consumer.OnInvalidBlockDetected(err)
	}
}
