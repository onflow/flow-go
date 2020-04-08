package notifications

import (
	"sync"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
)

// PubSubDistributor is an example implementation of notifications.Consumer
// that distributes notifications to a list of subscribers.
//
// It allows thread-safe subscription of multiple consumers to events.
type PubSubDistributor struct {
	skippedAheadConsumers          []SkippedAheadConsumer
	enteringViewConsumers          []EnteringViewConsumer
	startingTimeoutConsumers       []StartingTimeoutConsumer
	reachedTimeoutConsumers        []ReachedTimeoutConsumer
	qcIncorporatedConsumers        []QcIncorporatedConsumer
	forkChoiceGeneratedConsumers   []ForkChoiceGeneratedConsumer
	blockIncorporatedConsumers     []BlockIncorporatedConsumer
	finalizedBlockConsumers        []FinalizedBlockConsumer
	doubleProposeDetectedConsumers []DoubleProposeDetectedConsumer
	doubleVotingDetectedConsumers  []DoubleVotingDetectedConsumer
	invalidVoteDetectedConsumers   []InvalidVoteDetectedConsumer
	lock                           sync.RWMutex
}

func NewPubSubDistributor() *PubSubDistributor {
	return &PubSubDistributor{}
}

func (p *PubSubDistributor) OnSkippedAhead(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.skippedAheadConsumers {
		subscriber.OnSkippedAhead(view)
	}
}

func (p *PubSubDistributor) OnEnteringView(view uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.enteringViewConsumers {
		subscriber.OnEnteringView(view)
	}
}

func (p *PubSubDistributor) OnStartingTimeout(timerInfo *model.TimerInfo) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.startingTimeoutConsumers {
		subscriber.OnStartingTimeout(timerInfo)
	}
}

func (p *PubSubDistributor) OnReachedTimeout(timeout *model.TimerInfo) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.reachedTimeoutConsumers {
		subscriber.OnReachedTimeout(timeout)
	}
}

func (p *PubSubDistributor) OnQcIncorporated(qc *model.QuorumCertificate) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.qcIncorporatedConsumers {
		subscriber.OnQcIncorporated(qc)
	}
}

func (p *PubSubDistributor) OnForkChoiceGenerated(curView uint64, selectedQC *model.QuorumCertificate) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.forkChoiceGeneratedConsumers {
		subscriber.OnForkChoiceGenerated(curView, selectedQC)
	}
}

func (p *PubSubDistributor) OnBlockIncorporated(block *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.blockIncorporatedConsumers {
		subscriber.OnBlockIncorporated(block)
	}
}

func (p *PubSubDistributor) OnFinalizedBlock(block *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.finalizedBlockConsumers {
		subscriber.OnFinalizedBlock(block)
	}
}

func (p *PubSubDistributor) OnDoubleProposeDetected(block1, block2 *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.doubleProposeDetectedConsumers {
		subscriber.OnDoubleProposeDetected(block1, block2)
	}
}

func (p *PubSubDistributor) OnDoubleVotingDetected(vote1, vote2 *model.Vote) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.doubleVotingDetectedConsumers {
		subscriber.OnDoubleVotingDetected(vote1, vote2)
	}
}

func (p *PubSubDistributor) OnInvalidVoteDetected(vote *model.Vote) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.invalidVoteDetectedConsumers {
		subscriber.OnInvalidVoteDetected(vote)
	}
}

// AddSkippedAheadConsumer adds an SkippedAheadConsumer to the PubSubDistributor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubDistributor) AddSkippedAheadConsumer(cons SkippedAheadConsumer) *PubSubDistributor {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.skippedAheadConsumers = append(p.skippedAheadConsumers, cons)
	return p
}

// AddEnteringViewConsumer adds an EnteringViewConsumer to the PubSubDistributor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubDistributor) AddEnteringViewConsumer(cons EnteringViewConsumer) *PubSubDistributor {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.enteringViewConsumers = append(p.enteringViewConsumers, cons)
	return p
}

// AddStartingTimeoutConsumer adds an StartingTimeoutConsumer to the PubSubDistributor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubDistributor) AddStartingTimeoutConsumer(cons StartingTimeoutConsumer) *PubSubDistributor {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.startingTimeoutConsumers = append(p.startingTimeoutConsumers, cons)
	return p
}

// AddReachedTimeoutConsumer adds an ReachedTimeoutConsumer to the PubSubDistributor;
// concurrency safe; returns self-reference for chaining
func (p *PubSubDistributor) AddReachedTimeoutConsumer(cons ReachedTimeoutConsumer) *PubSubDistributor {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.reachedTimeoutConsumers = append(p.reachedTimeoutConsumers, cons)
	return p
}
