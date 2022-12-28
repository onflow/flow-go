package notifications

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// NoopConsumer is an implementation of the notifications consumer that
// doesn't do anything.
type NoopConsumer struct {
	NoopFinalizationConsumer
	NoopPartialConsumer
	NoopCommunicatorConsumer
}

var _ hotstuff.Consumer = (*NoopConsumer)(nil)

func NewNoopConsumer() *NoopConsumer {
	nc := &NoopConsumer{}
	return nc
}

// no-op implementation of hotstuff.Consumer(but not nested interfaces)

type NoopPartialConsumer struct{}

func (*NoopPartialConsumer) OnEventProcessed() {}

func (*NoopPartialConsumer) OnStart(uint64) {}

func (*NoopPartialConsumer) OnReceiveProposal(uint64, *model.Proposal) {}

func (*NoopPartialConsumer) OnReceiveQc(uint64, *flow.QuorumCertificate) {}

func (*NoopPartialConsumer) OnReceiveTc(uint64, *flow.TimeoutCertificate) {}

func (*NoopPartialConsumer) OnPartialTc(uint64, *hotstuff.PartialTcCreated) {}

func (*NoopPartialConsumer) OnLocalTimeout(uint64) {}

func (*NoopPartialConsumer) OnViewChange(uint64, uint64) {}

func (*NoopPartialConsumer) OnQcTriggeredViewChange(*flow.QuorumCertificate, uint64) {}

func (*NoopPartialConsumer) OnTcTriggeredViewChange(*flow.TimeoutCertificate, uint64) {}

func (*NoopPartialConsumer) OnStartingTimeout(model.TimerInfo) {}

func (*NoopPartialConsumer) OnVoteProcessed(*model.Vote) {}

func (*NoopPartialConsumer) OnTimeoutProcessed(*model.TimeoutObject) {}

func (*NoopPartialConsumer) OnCurrentViewDetails(uint64, flow.Identifier) {}

func (*NoopPartialConsumer) OnDoubleVotingDetected(*model.Vote, *model.Vote) {}

func (*NoopPartialConsumer) OnInvalidVoteDetected(*model.Vote) {}

func (*NoopPartialConsumer) OnVoteForInvalidBlockDetected(*model.Vote, *model.Proposal) {}

func (*NoopPartialConsumer) OnDoubleTimeoutDetected(*model.TimeoutObject, *model.TimeoutObject) {}

func (*NoopPartialConsumer) OnInvalidTimeoutDetected(*model.TimeoutObject) {}

// no-op implementation of hotstuff.FinalizationConsumer

type NoopFinalizationConsumer struct{}

var _ hotstuff.FinalizationConsumer = (*NoopFinalizationConsumer)(nil)

func (*NoopFinalizationConsumer) OnBlockIncorporated(*model.Block) {}

func (*NoopFinalizationConsumer) OnFinalizedBlock(*model.Block) {}

func (*NoopFinalizationConsumer) OnDoubleProposeDetected(*model.Block, *model.Block) {}

// no-op implementation of hotstuff.TimeoutCollectorConsumer

type NoopTimeoutCollectorConsumer struct{}

var _ hotstuff.TimeoutCollectorConsumer = (*NoopTimeoutCollectorConsumer)(nil)

func (*NoopTimeoutCollectorConsumer) OnTcConstructedFromTimeouts(*flow.TimeoutCertificate) {}

func (*NoopTimeoutCollectorConsumer) OnPartialTcCreated(uint64, *flow.QuorumCertificate, *flow.TimeoutCertificate) {
}

func (*NoopTimeoutCollectorConsumer) OnNewQcDiscovered(*flow.QuorumCertificate) {}

func (*NoopTimeoutCollectorConsumer) OnNewTcDiscovered(*flow.TimeoutCertificate) {}

// no-op implementation of hotstuff.CommunicatorConsumer

type NoopCommunicatorConsumer struct{}

var _ hotstuff.CommunicatorConsumer = (*NoopCommunicatorConsumer)(nil)

func (*NoopCommunicatorConsumer) OnOwnVote(flow.Identifier, uint64, []byte, flow.Identifier) {}

func (*NoopCommunicatorConsumer) OnOwnTimeout(*model.TimeoutObject) {}

func (*NoopCommunicatorConsumer) OnOwnProposal(*flow.Header, time.Time) {}

// no-op implementation of hotstuff.QCCreatedConsumer

type NoopQCCreatedConsumer struct{}

var _ hotstuff.QCCreatedConsumer = (*NoopQCCreatedConsumer)(nil)

func (*NoopQCCreatedConsumer) OnQcConstructedFromVotes(*flow.QuorumCertificate) {}
