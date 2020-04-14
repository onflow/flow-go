package hotstuff

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/runner"
	"github.com/dapperlabs/flow-go/module"
)

// EventLoop buffers all incoming events to the hotstuff EventHandler, and feeds EventHandler one event at a time.
type EventLoop struct {
	log          zerolog.Logger
	eventHandler EventHandler
	metrics      module.Metrics
	proposals    chan *model.Proposal
	votes        chan *model.Vote

	runner runner.SingleRunner // lock for preventing concurrent state transitions
}

// NewEventLoop creates an instance of EventLoop.
func NewEventLoop(log zerolog.Logger, metrics module.Metrics, eventHandler EventHandler) (*EventLoop, error) {
	proposals := make(chan *model.Proposal)
	votes := make(chan *model.Vote)

	el := &EventLoop{
		log:          log,
		eventHandler: eventHandler,
		metrics:      metrics,
		proposals:    proposals,
		votes:        votes,
		runner:       runner.NewSingleRunner(),
	}

	return el, nil
}

func (el *EventLoop) loop() {

	// hotstuff will run in an event loop to process all events synchronously. And this is what will happen when hitting errors:
	// if hotstuff hits a known critical error, it will exit the loop (for instance, there is a conflicting block with a QC against finalized blocks
	// if hotstuff hits a known error indicating some assumption between components is broken, it will exit the loop (for instance, hotstuff receives a block whose parent is missing)
	// if hotstuff hits a known error that is safe to be ignored, it will not exit the loop (for instance, double voting/invalid vote)
	// if hotstuff hits any unknown error, it will exit the loop

	for {
		shutdownSignal := el.runner.ShutdownSignal()

		// Giving timeout events the priority to be processed first
		// This is to prevent attacks from malicious nodes that attempt
		// to block honest nodes' pacemaker from progressing by sending
		// other events.
		timeoutChannel := el.eventHandler.TimeoutChannel()

		idleStart := time.Now()

		// the first select makes sure we process timeouts with priority
		select {

		// if we receive the shutdown signal, exit the loop
		case <-shutdownSignal:
			return

		// if we receive a time out, process it and log errors
		case t := <-timeoutChannel:

			// measure how long it takes for a timeout event to go through
			// eventloop and get handled
			busyDuration := time.Since(t)
			el.metrics.HotStuffBusyDuration(busyDuration)

			// meansure how long the event loop was idle waiting for an
			// incoming event
			idleDuration := time.Since(idleStart)
			el.metrics.HotStuffIdleDuration(idleDuration)

			err := el.eventHandler.OnLocalTimeout()
			if err != nil {
				el.log.Fatal().Err(err).Msg("could not process timeout")
			}

		default:
			// fall through to non-priority events
		}

		// select for block headers/votes here
		select {

		// same as before
		case <-shutdownSignal:
			return

		// same as before
		case t := <-timeoutChannel:
			busyDuration := time.Since(t)
			el.metrics.HotStuffBusyDuration(busyDuration)

			idleDuration := time.Since(idleStart)
			el.metrics.HotStuffBusyDuration(idleDuration)

			err := el.eventHandler.OnLocalTimeout()
			if err != nil {
				el.log.Fatal().Err(err).Msg("could not process timeout")
			}

		// if we have a new proposal, process it
		case p := <-el.proposals:
			idleDuration := time.Since(idleStart)
			el.metrics.HotStuffBusyDuration(idleDuration)

			err := el.eventHandler.OnReceiveProposal(p)
			if err != nil {
				el.log.Fatal().Err(err).Msg("could not process proposal")
			}

		// if we have a new vote, process it
		case v := <-el.votes:
			idleDuration := time.Since(idleStart)
			el.metrics.HotStuffBusyDuration(idleDuration)

			err := el.eventHandler.OnReceiveVote(v)
			if err != nil {
				el.log.Fatal().Err(err).Msg("could not process vote")
			}
		}
	}
}

// OnReceiveProposal pushes the received block to the blockheader channel
func (el *EventLoop) OnReceiveProposal(proposal *model.Proposal) {
	received := time.Now()

	el.proposals <- proposal

	// the busy duration is measured as how long it takes from a block being
	// received to a block being handled by the event handler.
	busyDuration := time.Since(received)
	el.metrics.HotStuffBusyDuration(busyDuration)
}

// OnReceiveVote pushes the received vote to the votes channel
func (el *EventLoop) OnReceiveVote(vote *model.Vote) {
	received := time.Now()

	el.votes <- vote

	// the busy duration is measured as how long it takes from a vote being
	// received to a vote being handled by the event handler.
	busyDuration := time.Since(received)
	el.metrics.HotStuffBusyDuration(busyDuration)
}

// Ready implements interface module.ReadyDoneAware
// Method call will starts the EventLoop's internal processing loop.
// Multiple calls are handled gracefully and the event loop will only start
// once.
func (el *EventLoop) Ready() <-chan struct{} {
	err := el.eventHandler.Start()
	if err != nil {
		el.log.Fatal().Err(err).Msg("could not start event handler")
	}
	return el.runner.Start(el.loop)
}

// Done implements interface module.ReadyDoneAware
func (el *EventLoop) Done() <-chan struct{} {
	return el.runner.Abort()
}

// Wait implements a function to wait for the event loop to exit.
func (el *EventLoop) Wait() <-chan struct{} {
	return el.runner.Completed()
}
