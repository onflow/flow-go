package hotstuff

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

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
// • Each time the EventHandler processes an event, the EventHandler might call into PaceMaker
//   potentially resulting in a state transition and the PaceMaker starting a new timeout
//
// • Hence, after processing any event, EventHandler should retrieve the current TimeoutChannel
//   from the PaceMaker.
//
// For Example:
//
// for {
//		timeoutChannel := el.eventHandler.TimeoutChannel()
//		select {
//		   case <-timeoutChannel:
//		    	el.eventHandler.OnLocalTimeout()
//		   case <other events>
//		}
// }
type PaceMaker interface {

	// CurView returns the current view.
	CurView() uint64

	// ProcessQC will check if the given QC will allow PaceMaker to fast-forward to QC.view+1.
	// If PaceMaker incremented the current View, a NewViewEvent will be returned.
	ProcessQC(qc *flow.QuorumCertificate) (*model.NewViewEvent, bool)

	// ProcessTC will check if the given TC will allow PaceMaker to fast-forward to TC.view+1.
	// If PaceMaker incremented the current View, a NewViewEvent will be returned.
	ProcessTC(tc *flow.TimeoutCertificate) (*model.NewViewEvent, bool)

	// TimeoutChannel returns the timeout channel for the CURRENTLY ACTIVE timeout.
	// Each time the pacemaker starts a new timeout, this channel is replaced.
	TimeoutChannel() <-chan time.Time

	// OnTimeout is called when a timeout, which was previously created by the PaceMaker, has
	// looped through the event loop. Triggering a timeout will result with high probability in creating a TimeoutObject.
	// It is the responsibility of the calling code to ensure that NO STALE timeouts are
	// delivered to the PaceMaker.
	OnTimeout()

	// OnPartialTC is called when TC collector will collect f+1 timeouts. This implements Bracha style timeouts,
	// which times out current view after receiving partial TC.
	OnPartialTC(curView uint64)

	// Start starts the PaceMaker (i.e. the timeout for the configured starting value for view).
	Start()

	// BlockRateDelay
	BlockRateDelay() time.Duration
}
