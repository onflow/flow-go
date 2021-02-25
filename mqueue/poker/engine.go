package poker

import (
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/mqueue"
	"github.com/onflow/flow-go/utils/fifoqueue"
)

// Engine shows the
type Engine struct {
	// queue of inbound messages to process, populated by network worker thread
	// calling ProcessMessageFromNetwork
	messages *fifoqueue.FifoQueue
	poker    Poker
	unit     *engine.Unit
}

// the network->queue step processor just adds the item to the queue.
func (e *Engine) ProcessMessageFromNetwork(message mqueue.Message) {
	e.messages.Push(message)
}

func (e *Engine) ProcessMessagesFromQueue() {
	for {
		select {
		case <-e.unit.Quit():
			return

		case <-e.poker.Wait():
			e.processMessagesWhileAvailable()
		}
	}
}

func (e *Engine) processMessagesWhileAvailable() {
	for {
		msg, ok := e.selectNextMessage()
		if !ok {
			return
		}
		e.processMessage(msg)
	}
}

// This can be a module separate from the engine/core, with read-only access to queue/pending state
// Eg. `e.MessageSelector.Next()`
func (e *Engine) selectNextMessage() (mqueue.Message, bool) {
	// here we can implement arbitrary prioritization logic based on
	// message type priority, our pending state, the size of all our
	// inbound queues, etc.
	return nil, true
}

// This represents the Core logic
func (e *Engine) processMessage(_ mqueue.Message) {
	// type switch, processing logic
}
