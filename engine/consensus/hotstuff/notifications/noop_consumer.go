package notifications

import (
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

// NoopConsumer is an implementation of the notifications consumer that
// doesn't do anything.
type NoopConsumer struct{}

func (NoopConsumer) OnBlockIncorporated(*hotstuff.Block) {}

func (NoopConsumer) OnFinalizedBlock(*hotstuff.Block) {}

func (NoopConsumer) OnDoubleProposeDetected(*hotstuff.Block, *hotstuff.Block) {}

func (NoopConsumer) OnEnteringView(uint64) {}

func (NoopConsumer) OnSkippedAhead(uint64) {}

func (NoopConsumer) OnStartingTimeout(*hotstuff.TimerInfo) {}

func (NoopConsumer) OnReachedTimeout(*hotstuff.TimerInfo) {}

func (NoopConsumer) OnQcIncorporated(*hotstuff.QuorumCertificate) {}

func (NoopConsumer) OnForkChoiceGenerated(uint64, *hotstuff.QuorumCertificate) {}

func (NoopConsumer) OnDoubleVotingDetected(*hotstuff.Vote, *hotstuff.Vote) {}

func (NoopConsumer) OnInvalidVoteDetected(*hotstuff.Vote) {}
