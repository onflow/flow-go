package eventloop

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/tracker"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
)

// queuedProposal is a helper structure that is used to transmit proposal in channel
// it contains an attached insertionTime that is used to measure how long we have waited between queening proposal and
// actually processing by `EventHandler`.
type queuedProposal struct {
	proposal      *model.Proposal
	insertionTime time.Time
}

// EventLoop buffers all incoming events to the hotstuff EventHandler, and feeds EventHandler one event at a time.
type EventLoop struct {
	*component.ComponentManager
	log                      zerolog.Logger
	eventHandler             hotstuff.EventHandler
	metrics                  module.HotstuffMetrics
	mempoolMetrics           module.MempoolMetrics
	proposals                chan queuedProposal
	newestSubmittedTc        *tracker.NewestTCTracker
	newestSubmittedQc        *tracker.NewestQCTracker
	newestSubmittedPartialTc *tracker.NewestPartialTcTracker
	tcSubmittedNotifier      engine.Notifier
	qcSubmittedNotifier      engine.Notifier
	partialTcCreatedNotifier engine.Notifier
	startTime                time.Time
}

var _ hotstuff.EventLoop = (*EventLoop)(nil)
var _ component.Component = (*EventLoop)(nil)

// NewEventLoop creates an instance of EventLoop.
func NewEventLoop(
	log zerolog.Logger,
	metrics module.HotstuffMetrics,
	mempoolMetrics module.MempoolMetrics,
	eventHandler hotstuff.EventHandler,
	startTime time.Time,
) (*EventLoop, error) {
	// we will use a buffered channel to avoid blocking of caller
	// we can't afford to drop messages since it undermines liveness, but we also want to avoid blocking of compliance
	// engine. We assume that we should be able to process proposals faster than compliance engine feeds them, worst case
	// we will fill the buffer and block compliance engine worker but that should happen only if compliance engine receives
	// large number of blocks in short period of time(when catching up for instance).
	proposals := make(chan queuedProposal, 1000)

	el := &EventLoop{
		log:                      log,
		eventHandler:             eventHandler,
		metrics:                  metrics,
		mempoolMetrics:           mempoolMetrics,
		proposals:                proposals,
		tcSubmittedNotifier:      engine.NewNotifier(),
		qcSubmittedNotifier:      engine.NewNotifier(),
		partialTcCreatedNotifier: engine.NewNotifier(),
		newestSubmittedTc:        tracker.NewNewestTCTracker(),
		newestSubmittedQc:        tracker.NewNewestQCTracker(),
		newestSubmittedPartialTc: tracker.NewNewestPartialTcTracker(),
		startTime:                startTime,
	}

	componentBuilder := component.NewComponentManagerBuilder()
	componentBuilder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready()

		// launch when scheduled by el.startTime
		el.log.Info().Msgf("event loop will start at: %v", startTime)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(startTime)):
			el.log.Info().Msgf("starting event loop")
			err := el.loop(ctx)
			if err != nil {
				el.log.Error().Err(err).Msg("irrecoverable event loop error")
				ctx.Throw(err)
			}
		}
	})
	el.ComponentManager = componentBuilder.Build()

	return el, nil
}

// loop executes the core HotStuff logic in a single thread. It picks inputs from the various
// inbound channels and executes the EventHandler's respective method for processing this input.
// During normal operations, the EventHandler is not expected to return any errors, as all inputs
// are assumed to be fully validated (or produced by trusted components within the node). Therefore,
// any error is a symptom of state corruption, bugs or violation of API contracts. In all cases,
// continuing operations is not an option, i.e. we exit the event loop and return an exception.
func (el *EventLoop) loop(ctx context.Context) error {
	err := el.eventHandler.Start(ctx) // must be called by the same go-routine that also executes the business logic!
	if err != nil {
		return fmt.Errorf("could not start event handler: %w", err)
	}

	shutdownSignaled := ctx.Done()
	timeoutCertificates := el.tcSubmittedNotifier.Channel()
	quorumCertificates := el.qcSubmittedNotifier.Channel()
	partialTCs := el.partialTcCreatedNotifier.Channel()

	for {
		// Giving timeout events the priority to be processed first.
		// This is to prevent attacks from malicious nodes that attempt
		// to block honest nodes' pacemaker from progressing by sending
		// other events.
		timeoutChannel := el.eventHandler.TimeoutChannel()

		// the first select makes sure we process timeouts with priority
		select {

		// if we receive the shutdown signal, exit the loop
		case <-shutdownSignaled:
			return nil

		// processing timeout or partial TC event are top priority since
		// they allow node to contribute to TC aggregation when replicas can't
		// make progress on happy path
		case <-timeoutChannel:

			processStart := time.Now()
			err = el.eventHandler.OnLocalTimeout()
			if err != nil {
				return fmt.Errorf("could not process timeout: %w", err)
			}
			// measure how long it takes for a timeout event to be processed
			el.metrics.HotStuffBusyDuration(time.Since(processStart), metrics.HotstuffEventTypeLocalTimeout)

			// At this point, we have received and processed an event from the timeout channel.
			// A timeout also means that we have made progress. A new timeout will have
			// been started and el.eventHandler.TimeoutChannel() will be a NEW channel (for the just-started timeout).
			// Very important to start the for loop from the beginning, to continue the with the new timeout channel!
			continue

		case <-partialTCs:

			processStart := time.Now()
			err = el.eventHandler.OnPartialTcCreated(el.newestSubmittedPartialTc.NewestPartialTc())
			if err != nil {
				return fmt.Errorf("could no process partial created TC event: %w", err)
			}
			// measure how long it takes for a partial TC to be processed
			el.metrics.HotStuffBusyDuration(time.Since(processStart), metrics.HotstuffEventTypeOnPartialTc)

			// At this point, we have received and processed partial TC event, it could have resulted in several scenarios:
			// 1. a view change with potential voting or proposal creation
			// 2. a created and broadcast timeout object
			// 3. QC and TC didn't result in view change and no timeout was created since we have already timed out or
			// the partial TC was created for view different from current one.
			continue

		default:
			// fall through to non-priority events
		}

		idleStart := time.Now()

		// select for block headers/QCs here
		select {

		// same as before
		case <-shutdownSignaled:
			return nil

		// same as before
		case <-timeoutChannel:
			// measure how long the event loop was idle waiting for an
			// incoming event
			el.metrics.HotStuffIdleDuration(time.Since(idleStart))

			processStart := time.Now()
			err = el.eventHandler.OnLocalTimeout()
			if err != nil {
				return fmt.Errorf("could not process timeout: %w", err)
			}
			// measure how long it takes for a timeout event to be processed
			el.metrics.HotStuffBusyDuration(time.Since(processStart), metrics.HotstuffEventTypeLocalTimeout)

		// if we have a new proposal, process it
		case queuedItem := <-el.proposals:
			// the wait duration is measured as how long it takes from a block being
			// received to event handler commencing the processing of the block
			el.metrics.HotStuffWaitDuration(time.Since(queuedItem.insertionTime), metrics.HotstuffEventTypeOnProposal)

			// measure how long the event loop was idle waiting for an
			// incoming event
			el.metrics.HotStuffIdleDuration(time.Since(idleStart))

			processStart := time.Now()
			proposal := queuedItem.proposal
			err = el.eventHandler.OnReceiveProposal(proposal)
			if err != nil {
				return fmt.Errorf("could not process proposal %v: %w", proposal.Block.BlockID, err)
			}
			// measure how long it takes for a proposal to be processed
			el.metrics.HotStuffBusyDuration(time.Since(processStart), metrics.HotstuffEventTypeOnProposal)

			el.log.Info().
				Dur("dur_ms", time.Since(processStart)).
				Uint64("view", proposal.Block.View).
				Hex("block_id", proposal.Block.BlockID[:]).
				Msg("block proposal has been processed successfully")

		// if we have a new QC, process it
		case <-quorumCertificates:
			// measure how long the event loop was idle waiting for an
			// incoming event
			el.metrics.HotStuffIdleDuration(time.Since(idleStart))

			processStart := time.Now()
			err = el.eventHandler.OnReceiveQc(el.newestSubmittedQc.NewestQC())
			if err != nil {
				return fmt.Errorf("could not process QC: %w", err)
			}
			// measure how long it takes for a QC to be processed
			el.metrics.HotStuffBusyDuration(time.Since(processStart), metrics.HotstuffEventTypeOnQC)

			// if we have a new TC, process it
		case <-timeoutCertificates:
			// measure how long the event loop was idle waiting for an
			// incoming event
			el.metrics.HotStuffIdleDuration(time.Since(idleStart))

			processStart := time.Now()
			err = el.eventHandler.OnReceiveTc(el.newestSubmittedTc.NewestTC())
			if err != nil {
				return fmt.Errorf("could not process TC: %w", err)
			}
			// measure how long it takes for a TC to be processed
			el.metrics.HotStuffBusyDuration(time.Since(processStart), metrics.HotstuffEventTypeOnTC)

		case <-partialTCs:
			// measure how long the event loop was idle waiting for an
			// incoming event
			el.metrics.HotStuffIdleDuration(time.Since(idleStart))

			processStart := time.Now()
			err = el.eventHandler.OnPartialTcCreated(el.newestSubmittedPartialTc.NewestPartialTc())
			if err != nil {
				return fmt.Errorf("could no process partial created TC event: %w", err)
			}
			// measure how long it takes for a partial TC to be processed
			el.metrics.HotStuffBusyDuration(time.Since(processStart), metrics.HotstuffEventTypeOnPartialTc)
		}
	}
}

// SubmitProposal pushes the received block to the proposals channel
func (el *EventLoop) SubmitProposal(proposal *model.Proposal) {
	queueItem := queuedProposal{
		proposal:      proposal,
		insertionTime: time.Now(),
	}
	select {
	case el.proposals <- queueItem:
	case <-el.ComponentManager.ShutdownSignal():
		return
	}
	el.mempoolMetrics.MempoolEntries(metrics.HotstuffEventTypeOnProposal, uint(len(el.proposals)))
}

// onTrustedQC pushes the received QC(which MUST be validated) to the quorumCertificates channel
func (el *EventLoop) onTrustedQC(qc *flow.QuorumCertificate) {
	if el.newestSubmittedQc.Track(qc) {
		el.qcSubmittedNotifier.Notify()
	}
}

// onTrustedTC pushes the received TC(which MUST be validated) to the timeoutCertificates channel
func (el *EventLoop) onTrustedTC(tc *flow.TimeoutCertificate) {
	if el.newestSubmittedTc.Track(tc) {
		el.tcSubmittedNotifier.Notify()
	} else if el.newestSubmittedQc.Track(tc.NewestQC) {
		el.qcSubmittedNotifier.Notify()
	}
}

// OnTcConstructedFromTimeouts pushes the received TC to the timeoutCertificates channel
func (el *EventLoop) OnTcConstructedFromTimeouts(tc *flow.TimeoutCertificate) {
	el.onTrustedTC(tc)
}

// OnPartialTcCreated created a hotstuff.PartialTcCreated payload and pushes it into partialTcCreated buffered channel for
// further processing by EventHandler. Since we use buffered channel this function can block if buffer is full.
func (el *EventLoop) OnPartialTcCreated(view uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) {
	event := &hotstuff.PartialTcCreated{
		View:       view,
		NewestQC:   newestQC,
		LastViewTC: lastViewTC,
	}
	if el.newestSubmittedPartialTc.Track(event) {
		el.partialTcCreatedNotifier.Notify()
	}
}

// OnNewQcDiscovered pushes already validated QCs that were submitted from TimeoutAggregator to the event handler
func (el *EventLoop) OnNewQcDiscovered(qc *flow.QuorumCertificate) {
	el.onTrustedQC(qc)
}

// OnNewTcDiscovered pushes already validated TCs that were submitted from TimeoutAggregator to the event handler
func (el *EventLoop) OnNewTcDiscovered(tc *flow.TimeoutCertificate) {
	el.onTrustedTC(tc)
}

// OnQcConstructedFromVotes implements hotstuff.VoteCollectorConsumer and pushes received qc into processing pipeline.
func (el *EventLoop) OnQcConstructedFromVotes(qc *flow.QuorumCertificate) {
	el.onTrustedQC(qc)
}

// OnTimeoutProcessed implements hotstuff.TimeoutCollectorConsumer and is no-op
func (el *EventLoop) OnTimeoutProcessed(timeout *model.TimeoutObject) {}

// OnVoteProcessed implements hotstuff.VoteCollectorConsumer and is no-op
func (el *EventLoop) OnVoteProcessed(vote *model.Vote) {}
