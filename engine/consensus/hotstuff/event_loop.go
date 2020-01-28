package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"go.uber.org/atomic"
)

type EventLoop struct {
	eventHandler *EventHandler

	blockheaders chan *types.BlockHeader
	votes        chan *types.Vote
	started      *atomic.Bool
}

func (el *EventLoop) loop() {
	for {

		// Giving timeout events the priority to be processed first
		// This is to prevent attacks from malicious nodes that attempt
		// to block honest nodes' pacemaker from progressing by sending
		// other events.
		timeoutChannel := el.eventHandler.TimeoutChannel()
		select {
		case <-timeoutChannel:
			el.eventHandler.OnLocalTimeout()
		case b := <-el.blockheaders:
			el.eventHandler.OnReceiveBlockHeader(b)
		case v := <-el.votes:
			el.eventHandler.OnReceiveVote(v)
		}
	}
}

func (el *EventLoop) OnReceiveBlockHeader(block *types.BlockHeader) {
	el.blockheaders <- block
}

func (el *EventLoop) OnReceiveVote(vote *types.Vote) {
	el.votes <- vote
}

func (el *EventLoop) Start() error {
	if el.started.Swap(true) {
		return nil
	}
	el.eventHandler.paceMaker.Start() // start Pacemaker;
	// Wait with starting EventLoop until Pacemaker is started, i.e. above call returned
	el.eventHandler.startNewView()
	el.loop()
	return nil
}
