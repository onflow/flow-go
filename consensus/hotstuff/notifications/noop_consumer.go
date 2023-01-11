package notifications

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// NoopConsumer is an implementation of the notifications consumer that
// doesn't do anything.
type NoopConsumer struct{}

var _ hotstuff.Consumer = (*NoopConsumer)(nil)

func NewNoopConsumer() *NoopConsumer {
	nc := &NoopConsumer{}
	return nc
}

func (*NoopConsumer) OnEventProcessed() {}

func (*NoopConsumer) OnBlockIncorporated(*model.Block) {}

func (*NoopConsumer) OnFinalizedBlock(*model.Block) {}

func (*NoopConsumer) OnDoubleProposeDetected(*model.Block, *model.Block) {}

func (c *NoopConsumer) OnReceiveVote(uint64, *model.Vote) {}

func (c *NoopConsumer) OnReceiveProposal(uint64, *model.Proposal) {}

func (*NoopConsumer) OnEnteringView(uint64, flow.Identifier) {}

func (c *NoopConsumer) OnQcTriggeredViewChange(*flow.QuorumCertificate, uint64) {}

func (c *NoopConsumer) OnProposingBlock(*model.Proposal) {}

func (c *NoopConsumer) OnVoting(*model.Vote) {}

func (c *NoopConsumer) OnQcConstructedFromVotes(curView uint64, qc *flow.QuorumCertificate) {}

func (*NoopConsumer) OnStartingTimeout(*model.TimerInfo) {}

func (*NoopConsumer) OnReachedTimeout(*model.TimerInfo) {}

func (*NoopConsumer) OnQcIncorporated(*flow.QuorumCertificate) {}

func (*NoopConsumer) OnForkChoiceGenerated(uint64, *flow.QuorumCertificate) {}

func (*NoopConsumer) OnDoubleVotingDetected(*model.Vote, *model.Vote) {}

func (*NoopConsumer) OnInvalidVoteDetected(*model.Vote) {}

func (*NoopConsumer) OnVoteForInvalidBlockDetected(*model.Vote, *model.Proposal) {}
