package eventloop

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/module/metrics"
)

// EventLoopV2 buffers all incoming events to the hotstuff EventHandler, and feeds EventHandler one event at a time.
type EventLoopV2 struct {
	log                zerolog.Logger
	eventHandler       hotstuff.EventHandlerV2
	metrics            module.HotstuffMetrics
	proposals          chan *model.Proposal
	quorumCertificates chan *flow.QuorumCertificate

	lm   *lifecycle.LifecycleManager
	unit *engine.Unit // lock for preventing concurrent state transitions
}

var _ hotstuff.EventLoopV2 = &EventLoopV2{}
var _ module.ReadyDoneAware = &EventLoopV2{}

// NewEventLoopV2 creates an instance of EventLoopV2.
func NewEventLoopV2(log zerolog.Logger, metrics module.HotstuffMetrics, eventHandler hotstuff.EventHandlerV2) (*EventLoopV2, error) {
	proposals := make(chan *model.Proposal)
	quorumCertificates := make(chan *flow.QuorumCertificate)

	el := &EventLoopV2{
		log:                log,
		lm:                 lifecycle.NewLifecycleManager(),
		eventHandler:       eventHandler,
		metrics:            metrics,
		proposals:          proposals,
		quorumCertificates: quorumCertificates,
		unit:               engine.NewUnit(),
	}

	return el, nil
}

func (el *EventLoopV2) loop() {

	err := el.eventHandler.Start()
	if err != nil {
		el.log.Fatal().Err(err).Msg("could not start event handler")
	}

	// hotstuff will run in an event loop to process all events synchronously. And this is what will happen when hitting errors:
	// if hotstuff hits a known critical error, it will exit the loop (for instance, there is a conflicting block with a QC against finalized blocks
	// if hotstuff hits a known error indicating some assumption between components is broken, it will exit the loop (for instance, hotstuff receives a block whose parent is missing)
	// if hotstuff hits a known error that is safe to be ignored, it will not exit the loop (for instance, invalid proposal)
	// if hotstuff hits any unknown error, it will exit the loop

	for {
		quitted := el.unit.Quit()

		// Giving timeout events the priority to be processed first
		// This is to prevent attacks from malicious nodes that attempt
		// to block honest nodes' pacemaker from progressing by sending
		// other events.
		timeoutChannel := el.eventHandler.TimeoutChannel()

		// the first select makes sure we process timeouts with priority
		select {

		// if we receive the shutdown signal, exit the loop
		case <-quitted:
			return

		// if we receive a time out, process it and log errors
		case <-timeoutChannel:

			processStart := time.Now()

			err := el.eventHandler.OnLocalTimeout()

			// measure how long it takes for a timeout event to be processed
			el.metrics.HotStuffBusyDuration(time.Since(processStart), metrics.HotstuffEventTypeTimeout)

			if err != nil {
				el.log.Fatal().Err(err).Msg("could not process timeout")
			}

			// At this point, we have received and processed an event from the timeout channel.
			// A timeout also means, we have made progress. A new timeout will have
			// been started and el.eventHandler.TimeoutChannel() will be a NEW channel (for the just-started timeout)
			// Very important to start the for loop from the beginning, to continue the with the new timeout channel!
			continue

		default:
			// fall through to non-priority events
		}

		idleStart := time.Now()

		// select for block headers/QCs here
		select {

		// same as before
		case <-quitted:
			return

		// same as before
		case <-timeoutChannel:
			// measure how long the event loop was idle waiting for an
			// incoming event
			el.metrics.HotStuffIdleDuration(time.Since(idleStart))

			processStart := time.Now()

			err := el.eventHandler.OnLocalTimeout()

			// measure how long it takes for a timeout event to be processed
			el.metrics.HotStuffBusyDuration(time.Since(processStart), metrics.HotstuffEventTypeTimeout)

			if err != nil {
				el.log.Fatal().Err(err).Msg("could not process timeout")
			}

		// if we have a new proposal, process it
		case p := <-el.proposals:
			// measure how long the event loop was idle waiting for an
			// incoming event
			el.metrics.HotStuffIdleDuration(time.Since(idleStart))

			processStart := time.Now()

			err := el.eventHandler.OnReceiveProposal(p)

			// measure how long it takes for a proposal to be processed
			el.metrics.HotStuffBusyDuration(time.Since(processStart), metrics.HotstuffEventTypeOnProposal)

			if err != nil {
				el.log.Fatal().Err(err).Msg("could not process proposal")
			}

		// if we have a new QC, process it
		case qc := <-el.quorumCertificates:
			// measure how long the event loop was idle waiting for an
			// incoming event
			el.metrics.HotStuffIdleDuration(time.Since(idleStart))

			processStart := time.Now()

			err := el.eventHandler.OnQCConstructed(qc)

			// measure how long it takes for a QC to be processed
			el.metrics.HotStuffBusyDuration(time.Since(processStart), metrics.HotstuffEventTypeOnQc)

			if err != nil {
				el.log.Fatal().Err(err).Msg("could not process QC")
			}
		}
	}
}

// SubmitProposal pushes the received block to the blockheader channel
func (el *EventLoopV2) SubmitProposal(proposalHeader *flow.Header, parentView uint64) {
	received := time.Now()

	proposal := model.ProposalFromFlow(proposalHeader, parentView)

	select {
	case el.proposals <- proposal:
	case <-el.unit.Quit():
		return
	}

	// the wait duration is measured as how long it takes from a block being
	// received to event handler commencing the processing of the block
	el.metrics.HotStuffWaitDuration(time.Since(received), metrics.HotstuffEventTypeOnProposal)
}

// SubmitTrustedQC pushes the received QC to the quorumCertificates channel
func (el *EventLoopV2) SubmitTrustedQC(qc *flow.QuorumCertificate) {
	received := time.Now()

	select {
	case el.quorumCertificates <- qc:
	case <-el.unit.Quit():
		return
	}

	// the wait duration is measured as how long it takes from a qc being
	// received to event handler commencing the processing of the qc
	el.metrics.HotStuffWaitDuration(time.Since(received), metrics.HotstuffEventTypeOnQc)
}

// Ready implements interface module.ReadyDoneAware
// Method call will starts the EventLoopV2's internal processing loop.
// Multiple calls are handled gracefully and the event loop will only start
// once.
func (el *EventLoopV2) Ready() <-chan struct{} {
	el.lm.OnStart(func() {
		el.unit.Launch(el.loop)
	})
	return el.lm.Started()
}

// Done implements interface module.ReadyDoneAware
func (el *EventLoopV2) Done() <-chan struct{} {
	el.lm.OnStop(func() {
		// wait for event loop to exit
		<-el.unit.Done()
	})
	return el.lm.Stopped()
}
