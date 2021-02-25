package rtqueue

import (
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/mqueue"
)

// Engine shows the structure of an engine using a real-time queue with channel
// based interface for outbound (queue->engine) messages.
type Engine struct {
	// queue of inbound messages to process, populated by network worker thread
	// calling ProcessMessageFromNetwork
	messages Queue
	unit     *engine.Unit
}

func (e *Engine) ProcessMessageFromNetwork(message mqueue.Message) {
	e.messages.Add(message)
}

func (e *Engine) ProcessMessagesFromQueue() {
	for {
		select {
		case <-e.unit.Quit():
			return

		case msg := <-e.messages.Recv():
			// NOTE: if the engine has multiple input message queues, they would
			// be additional case statements here
			//
			// CAUTION: using select directly makes it more difficult to prioritize messages
			e.processMessage(msg)
		}
	}
}

func (e *Engine) processMessage(_ mqueue.Message) {
	// type switch, processing logic
}
