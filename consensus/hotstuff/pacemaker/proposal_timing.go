package pacemaker

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
)

// StaticProposalDurationProvider is a hotstuff.ProposalDurationProvider which provides a static ProposalDuration.
type StaticProposalDurationProvider struct {
	dur time.Duration
}

var _ hotstuff.ProposalDurationProvider = (*StaticProposalDurationProvider)(nil)

func NewStaticProposalDurationProvider(dur time.Duration) StaticProposalDurationProvider {
	return StaticProposalDurationProvider{dur: dur}
}

func (p StaticProposalDurationProvider) TargetPublicationTime(_ uint64, timeViewEntered time.Time, _ flow.Identifier) time.Time {
	return timeViewEntered.Add(p.dur)
}

func NoProposalDelay() StaticProposalDurationProvider {
	return NewStaticProposalDurationProvider(0)
}
