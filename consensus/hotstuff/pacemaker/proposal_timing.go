package pacemaker

import (
	"github.com/onflow/flow-go/model/flow"
	"time"
)

// ProposalDurationProvider provides the ProposalDelay to the Pacemaker.
// The ProposalDelay is the time a leader should attempt to consume between
// entering a view and broadcasting its proposal for that view.
type ProposalDurationProvider interface {
	// TargetPublicationTime is intended to be called by the EventHandler, whenever it
	// wants to publish a new proposal. The event handler inputs
	//  - proposalView: the view it is proposing for,
	//  - timeViewEntered: the time when the EventHandler entered this view
	//  - parentBlockId: the ID of the parent block , which the EventHandler is building on
	// TargetPublicationTime returns the time stamp when the new proposal should be broadcasted.
	// For a given view where we are the primary, suppose the actual time we are done building our proposal is P:
	//   - if P < TargetPublicationTime(..), then the EventHandler should wait until
	//     `TargetPublicationTime` to broadcast the proposal
	//   - if P >= TargetPublicationTime(..), then the EventHandler should immediately broadcast the proposal
	// Concurrency safe.
	TargetPublicationTime(proposalView uint64, timeViewEntered time.Time, parentBlockId flow.Identifier) time.Time
}

// StaticProposalDurationProvider is a ProposalDurationProvider which provides a static ProposalDuration.
type StaticProposalDurationProvider struct {
	dur time.Duration
}

var _ ProposalDurationProvider = (*StaticProposalDurationProvider)(nil)

func NewStaticProposalDurationProvider(dur time.Duration) StaticProposalDurationProvider {
	return StaticProposalDurationProvider{dur: dur}
}

func (p StaticProposalDurationProvider) TargetPublicationTime(_ uint64, timeViewEntered time.Time, _ flow.Identifier) time.Time {
	return timeViewEntered.Add(p.dur)
}

func NoProposalDelay() StaticProposalDurationProvider {
	return NewStaticProposalDurationProvider(0)
}
