package hotstuff

import (
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// EventLoop buffers all incoming events to the hotstuff EventHandler, and feeds EventHandler one event at a time.
type EventLoop struct {
	log          zerolog.Logger
	eventHandler *EventHandler
	blockheaders chan *types.BlockHeader
	votes        chan *types.Vote
	started      *atomic.Bool
}

// NewEventLoop creates an instance of EventLoop
func NewEventLoop(log zerolog.Logger, eventHandler *EventHandler) (*EventLoop, error) {
	blockheaders := make(chan *types.BlockHeader)
	votes := make(chan *types.Vote)

	el := &EventLoop{
		log:          log,
		eventHandler: eventHandler,
		blockheaders: blockheaders,
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
	var err error
	select {
	case <-timeoutChannel:
		err = el.eventHandler.OnLocalTimeout()
	default:
	}
	if err != nil {
		return err
	}

	// select for block headers/votes here
	select {
	case <-timeoutChannel:
		err = el.eventHandler.OnLocalTimeout()
	case b := <-el.blockheaders:
		err = el.eventHandler.OnReceiveBlockHeader(b)
	case v := <-el.votes:
		err = el.eventHandler.OnReceiveVote(v)
	}
	return err
}

// OnReceiveBlockHeader pushes the received block to the blockheader channel
func (el *EventLoop) OnReceiveBlockHeader(block *types.BlockHeader) {
	el.blockheaders <- block
}

// OnReceiveVote pushes the received vote to the votes channel
func (el *EventLoop) OnReceiveVote(vote *types.Vote) {
	el.votes <- vote
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
