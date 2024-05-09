package hotstuff

import (
	"context"
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

type LivenessData struct {
	// CurrentView is the currently active view tracked by the PaceMaker. It is updated
	// whenever the PaceMaker sees evidence (QC or TC) for advancing to next view.
	CurrentView uint64
	// NewestQC is the newest QC (by view) observed by the PaceMaker. The QC can be observed on its own or as a part of TC.
	NewestQC *flow.QuorumCertificate
	// LastViewTC is the TC for the prior view (CurrentView-1), if this view timed out. If the previous round
	// ended with a QC, this QC is stored in NewestQC and LastViewTC is nil.
	LastViewTC *flow.TimeoutCertificate
}

// PaceMaker for HotStuff. The component is passive in that it only reacts to method calls.
// The PaceMaker does not perform state transitions on its own. Timeouts are emitted through
// channels. Each timeout has its own dedicated channel, which is garbage collected after the
// respective state has been passed. It is the EventHandler's responsibility to pick up
// timeouts from the currently active TimeoutChannel process them first and subsequently inform the
// PaceMaker about processing the timeout. Specifically, the intended usage pattern for the
// TimeoutChannels is as follows:
//
// • Each time the PaceMaker starts a new timeout, it created a new TimeoutChannel
//
// • The channel for the CURRENTLY ACTIVE timeout is returned by PaceMaker.TimeoutChannel()
//
//   - Each time the EventHandler processes an event, the EventHandler might call into PaceMaker
//     potentially resulting in a state transition and the PaceMaker starting a new timeout
//
//   - Hence, after processing any event, EventHandler should retrieve the current TimeoutChannel
//     from the PaceMaker.
//
// For Example:
//
//	for {
//		timeoutChannel := el.eventHandler.TimeoutChannel()
//		select {
//		   case <-timeoutChannel:
//		    	el.eventHandler.OnLocalTimeout()
//		   case <other events>
//		}
//	}
//
// Not concurrency safe.
type PaceMaker interface {
	ProposalDurationProvider

	// CurView returns the current view.
	CurView() uint64

	// NewestQC returns QC with the highest view discovered by PaceMaker.
	NewestQC() *flow.QuorumCertificate

	// LastViewTC returns TC for last view, this could be nil if previous round
	// has entered with a QC.
	LastViewTC() *flow.TimeoutCertificate

	// ProcessQC will check if the given QC will allow PaceMaker to fast-forward to QC.view+1.
	// If PaceMaker incremented the current View, a NewViewEvent will be returned.
	// No errors are expected during normal operation.
	ProcessQC(qc *flow.QuorumCertificate) (*model.NewViewEvent, error)

	// ProcessTC will check if the given TC will allow PaceMaker to fast-forward to TC.view+1.
	// If PaceMaker incremented the current View, a NewViewEvent will be returned.
	// A nil TC is an expected valid input.
	// No errors are expected during normal operation.
	ProcessTC(tc *flow.TimeoutCertificate) (*model.NewViewEvent, error)

	// TimeoutChannel returns the timeout channel for the CURRENTLY ACTIVE timeout.
	// Each time the pacemaker starts a new timeout, this channel is replaced.
	TimeoutChannel() <-chan time.Time

	// Start starts the PaceMaker (i.e. the timeout for the configured starting value for view).
	// CAUTION: EventHandler is not concurrency safe. The Start method must
	// be executed by the same goroutine that also calls the other business logic
	// methods, or concurrency safety has to be implemented externally.
	Start(ctx context.Context)
}

// ProposalDurationProvider generates the target publication time for block proposals.
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
