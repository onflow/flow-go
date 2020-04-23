package hotstuff

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/runner"
	"github.com/dapperlabs/flow-go/model/flow"
)

// EventLoop buffers all incoming events to the hotstuff EventHandler, and feeds EventHandler one event at a time.
type EventLoop struct {
	log    zerolog.Logger
	runner runner.SingleRunner // lock for preventing concurrent state transitions
}

// NewEventLoop creates an instance of EventLoop.
func NewEventLoop(log zerolog.Logger) (*EventLoop, error) {
	el := &EventLoop{
		log:    log,
		runner: runner.NewSingleRunner(),
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

		// the first select makes sure we process timeouts with priority
		select {

		// if we receive the shutdown signal, exit the loop
		case <-shutdownSignal:
			return
		}
	}
}

// SubmitProposal pushes the received block to the blockheader channel
func (el *EventLoop) SubmitProposal(proposalHeader *flow.Header, parentView uint64) {
}

// SubmitVote pushes the received vote to the votes channel
func (el *EventLoop) SubmitVote(originID flow.Identifier, blockID flow.Identifier, view uint64, sigData []byte) {
}

// Ready implements interface module.ReadyDoneAware
// Method call will starts the EventLoop's internal processing loop.
// Multiple calls are handled gracefully and the event loop will only start
// once.
func (el *EventLoop) Ready() <-chan struct{} {
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
