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
	NoopProposalViolationConsumer
	NoopFinalizationConsumer
	NoopParticipantConsumer
	NoopCommunicatorConsumer
}

var _ hotstuff.Consumer = (*NoopConsumer)(nil)

func NewNoopConsumer() *NoopConsumer {
	nc := &NoopConsumer{}
	return nc
}

// no-op implementation of hotstuff.Consumer(but not nested interfaces)

type NoopParticipantConsumer struct{}

func (*NoopParticipantConsumer) OnEventProcessed() {}

func (*NoopParticipantConsumer) OnStart(uint64) {}

func (*NoopParticipantConsumer) OnReceiveProposal(uint64, *model.Proposal) {}

func (*NoopParticipantConsumer) OnReceiveQc(uint64, *flow.QuorumCertificate) {}

func (*NoopParticipantConsumer) OnReceiveTc(uint64, *flow.TimeoutCertificate) {}

func (*NoopParticipantConsumer) OnPartialTc(uint64, *hotstuff.PartialTcCreated) {}

func (*NoopParticipantConsumer) OnLocalTimeout(uint64) {}

func (*NoopParticipantConsumer) OnViewChange(uint64, uint64) {}

func (*NoopParticipantConsumer) OnQcTriggeredViewChange(uint64, uint64, *flow.QuorumCertificate) {}

func (*NoopParticipantConsumer) OnTcTriggeredViewChange(uint64, uint64, *flow.TimeoutCertificate) {}

func (*NoopParticipantConsumer) OnStartingTimeout(model.TimerInfo) {}

func (*NoopParticipantConsumer) OnCurrentViewDetails(uint64, uint64, flow.Identifier) {}

// no-op implementation of hotstuff.FinalizationConsumer

type NoopFinalizationConsumer struct{}

var _ hotstuff.FinalizationConsumer = (*NoopFinalizationConsumer)(nil)

func (*NoopFinalizationConsumer) OnBlockIncorporated(*model.Block) {}

func (*NoopFinalizationConsumer) OnFinalizedBlock(*model.Block) {}

// no-op implementation of hotstuff.TimeoutCollectorConsumer

type NoopTimeoutCollectorConsumer struct{}

var _ hotstuff.TimeoutCollectorConsumer = (*NoopTimeoutCollectorConsumer)(nil)

func (*NoopTimeoutCollectorConsumer) OnTcConstructedFromTimeouts(*flow.TimeoutCertificate) {}

func (*NoopTimeoutCollectorConsumer) OnPartialTcCreated(uint64, *flow.QuorumCertificate, *flow.TimeoutCertificate) {
}

func (*NoopTimeoutCollectorConsumer) OnNewQcDiscovered(*flow.QuorumCertificate) {}

func (*NoopTimeoutCollectorConsumer) OnNewTcDiscovered(*flow.TimeoutCertificate) {}

func (*NoopTimeoutCollectorConsumer) OnTimeoutProcessed(*model.TimeoutObject) {}

// no-op implementation of hotstuff.CommunicatorConsumer

type NoopCommunicatorConsumer struct{}

var _ hotstuff.CommunicatorConsumer = (*NoopCommunicatorConsumer)(nil)

func (*NoopCommunicatorConsumer) OnOwnVote(flow.Identifier, uint64, []byte, flow.Identifier) {}

func (*NoopCommunicatorConsumer) OnOwnTimeout(*model.TimeoutObject) {}

func (*NoopCommunicatorConsumer) OnOwnProposal(*flow.Header, time.Time) {}

// no-op implementation of hotstuff.VoteCollectorConsumer

type NoopVoteCollectorConsumer struct{}

var _ hotstuff.VoteCollectorConsumer = (*NoopVoteCollectorConsumer)(nil)

func (*NoopVoteCollectorConsumer) OnQcConstructedFromVotes(*flow.QuorumCertificate) {}

func (*NoopVoteCollectorConsumer) OnVoteProcessed(*model.Vote) {}

// no-op implementation of hotstuff.ProposalViolationConsumer

type NoopProposalViolationConsumer struct{}

var _ hotstuff.ProposalViolationConsumer = (*NoopProposalViolationConsumer)(nil)

func (*NoopProposalViolationConsumer) OnInvalidBlockDetected(flow.Slashable[model.InvalidProposalError]) {
}

func (*NoopProposalViolationConsumer) OnDoubleProposeDetected(*model.Block, *model.Block) {}

func (*NoopProposalViolationConsumer) OnDoubleVotingDetected(*model.Vote, *model.Vote) {}

func (*NoopProposalViolationConsumer) OnInvalidVoteDetected(model.InvalidVoteError) {}

func (*NoopProposalViolationConsumer) OnVoteForInvalidBlockDetected(*model.Vote, *model.Proposal) {}

func (*NoopProposalViolationConsumer) OnDoubleTimeoutDetected(*model.TimeoutObject, *model.TimeoutObject) {
}

func (*NoopProposalViolationConsumer) OnInvalidTimeoutDetected(model.InvalidTimeoutError) {}
