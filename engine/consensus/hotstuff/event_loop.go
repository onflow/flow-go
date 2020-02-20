package hotstuff

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/dapperlabs/flow-go/model/hotstuff"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// EventLoop buffers all incoming events to the hotstuff EventHandler, and feeds EventHandler one event at a time.
type EventLoop struct {
	log          zerolog.Logger
	eventHandler *EventHandler
	proposals    chan *hotstuff.Proposal
	votes        chan *hotstuff.Vote
	started      *atomic.Bool
}

// NewEventLoop creates an instance of EventLoop
func NewEventLoop(log zerolog.Logger, eventHandler *EventHandler) (*EventLoop, error) {
	proposals := make(chan *hotstuff.Proposal)
	votes := make(chan *hotstuff.Vote)

	el := &EventLoop{
		log:          log,
		eventHandler: eventHandler,
		proposals:    proposals,
		votes:        votes,
		started:      atomic.NewBool(false),
	}

	return el, nil
}

func (el *EventLoop) loop() error {
	for {
		err := el.processEvent()
		// hotstuff will run in an event loop to process all events synchronously. And this is what will happen when hitting errors:
		// if hotstuff hits a known critical error, it will exit the loop (for instance, there is a conflicting block with a QC against finalized blocks
		// if hotstuff hits a known error indicates some assumption between components is broken, it will exit the loop (for instance, hotstuff receives a block whose parent is missing)
		// if hotstuff hits a known error that is safe to be ignored, it will not exit the loop (for instance, double voting/invalid vote)
		// if hotstuff hits any unknown error, it will exit the loop
		if err != nil {
			return err
		}
	}
}

// processEvent processes one event at a time.
// This function should only be called within the `loop` function
func (el *EventLoop) processEvent() error {
	// Giving timeout events the priority to be processed first
	// This is to prevent attacks from malicious nodes that attempt
	// to block honest nodes' pacemaker from progressing by sending
	// other events.
	timeoutChannel := el.eventHandler.TimeoutChannel()

	idleStart := time.Now()

	var err error
	select {
	case t := <-timeoutChannel:
		// measure how long it takes for a timeout event to go through
		// eventloop and get handled
		busyDuration := time.Now().Sub(t)
		el.log.Debug().Dur("busy_duration", busyDuration).
			Msg("busy duration to handle local timeout")

		// meansure how long the event loop was idle waiting for an
		// incoming event
		idleDuration := time.Now().Sub(idleStart)
		el.log.Debug().Dur("idle_duration", idleDuration)

		err = el.eventHandler.OnLocalTimeout()
	default:
	}

	if err != nil {
		return err
	}

	// select for block headers/votes here
	select {
	case t := <-timeoutChannel:
		busyDuration := time.Now().Sub(t)
		el.log.Debug().Dur("busy_duration", busyDuration).
			Msg("busy duration to handle local timeout")

		idleDuration := time.Now().Sub(idleStart)
		el.log.Debug().Dur("idle_duration", idleDuration)

		err = el.eventHandler.OnLocalTimeout()
	case p := <-el.proposals:
		idleDuration := time.Now().Sub(idleStart)
		el.log.Debug().Dur("idle_duration", idleDuration)

		err = el.eventHandler.OnReceiveProposal(p)
	case v := <-el.votes:
		idleDuration := time.Now().Sub(idleStart)
		el.log.Debug().Dur("idle_duration", idleDuration)

		err = el.eventHandler.OnReceiveVote(v)
	}
	return err
}

// OnReceiveProposal pushes the received block to the blockheader channel
func (el *EventLoop) OnReceiveProposal(proposal *hotstuff.Proposal) {
	received := time.Now()

	el.proposals <- proposal

	// the busy duration is measured as how long it takes from a block being
	// received to a block being handled by the event handler.
	busyDuration := time.Now().Sub(received)
	el.log.Debug().Hex("block_ID", logging.ID(proposal.Block.BlockID)).
		Uint64("view", proposal.Block.View).
		Dur("busy_duration", busyDuration).
		Msg("busy duration to handle a proposal")
}

// OnReceiveVote pushes the received vote to the votes channel
func (el *EventLoop) OnReceiveVote(vote *hotstuff.Vote) {
	received := time.Now()

	el.votes <- vote

	// the busy duration is measured as how long it takes from a vote being
	// received to a vote being handled by the event handler.
	busyDuration := time.Now().Sub(received)
	el.log.Debug().Hex("vote_id", logging.ID(vote.BlockID)).
		Uint64("view", vote.View).
		Dur("busy_duration", busyDuration).
		Msg("busy duration to handle a vote")
}

// Start will start the event handler then enter the loop
func (el *EventLoop) Start() error {
	if el.started.Swap(true) {
		return nil
	}
	err := el.eventHandler.Start()
	if err != nil {
		return fmt.Errorf("can not start the eventloop: %w", err)
	}
	return el.loop()
}
