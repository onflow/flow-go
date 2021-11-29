package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
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

var _ hotstuff.Consumer = (*QCCreatedDistributor)(nil)

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

func (*QCCreatedDistributor) OnBlockIncorporated(*model.Block) {

}

func (*QCCreatedDistributor) OnFinalizedBlock(*model.Block) {

}

func (*QCCreatedDistributor) OnDoubleProposeDetected(*model.Block, *model.Block) {

}

func (*QCCreatedDistributor) OnEventProcessed() {

}

func (*QCCreatedDistributor) OnReceiveVote(uint64, *model.Vote) {

}

func (*QCCreatedDistributor) OnReceiveProposal(uint64, *model.Proposal) {

}

func (*QCCreatedDistributor) OnEnteringView(uint64, flow.Identifier) {

}

func (*QCCreatedDistributor) OnQcTriggeredViewChange(*flow.QuorumCertificate, uint64) {

}

func (*QCCreatedDistributor) OnProposingBlock(*model.Proposal) {

}

func (*QCCreatedDistributor) OnVoting(*model.Vote) {

}

func (*QCCreatedDistributor) OnStartingTimeout(*model.TimerInfo) {
}

func (*QCCreatedDistributor) OnReachedTimeout(*model.TimerInfo) {
}

func (*QCCreatedDistributor) OnQcIncorporated(*flow.QuorumCertificate) {
}

func (*QCCreatedDistributor) OnForkChoiceGenerated(uint64, *flow.QuorumCertificate) {
}

func (*QCCreatedDistributor) OnDoubleVotingDetected(*model.Vote, *model.Vote) {
}

func (*QCCreatedDistributor) OnInvalidVoteDetected(*model.Vote) {
}

func (*QCCreatedDistributor) OnVoteForInvalidBlockDetected(*model.Vote, *model.Proposal) {
}
