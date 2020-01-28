package stub

import (
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
)

// PendingMessage is a pending message to be sent
type PendingMessage struct {
	// The sender node id
	From      flow.Identifier
	ChannelID uint8
	Event     interface{}
	// The id of the receiver nodes
	TargetIDs []flow.Identifier
}

// Buffer buffers all the pending messages to be sent over the mock network from one node to a list of nodes
type Buffer struct {
	sync.Mutex
	pending []*PendingMessage
}

// NewBuffer initialize the Buffer
func NewBuffer() *Buffer {
	return &Buffer{
		pending: make([]*PendingMessage, 0),
	}
}

// Save stores a pending message to the buffer
func (b *Buffer) Save(m *PendingMessage) {
	b.Lock()
	defer b.Unlock()

	b.pending = append(b.pending, m)
}

// DeliverRecursive recursively delivers all pending messages using the provided
// sendOne method until the buffer is empty. If sendOne does not deliver the
// message, it is permanently dropped.
func (b *Buffer) DeliverRecursive(sendOne func(*PendingMessage)) {
	for {
		// get all pending messages, and clear the buffer
		messages := b.takeAll()

		// This check is necessary to exit the endless for loop
		if len(messages) == 0 {
			return
		}

		for _, msg := range messages {
			sendOne(msg)
		}
	}
}

// Deliver delivers all pending messages currently in the buffer using the
// provided sendOne method. If sendOne returns false, the message was not sent
// and will remain in the buffer.
func (b *Buffer) Deliver(sendOne func(*PendingMessage) bool) {

	messages := b.takeAll()
	var unsent []*PendingMessage

	for _, msg := range messages {
		ok := sendOne(msg)
		if !ok {
			unsent = append(unsent, msg)
		}
	}

	// add the unsent messages back to the buffer
	b.Lock()
	b.pending = append(unsent, b.pending...)
	b.Unlock()
}

// takeAll takes all pending messages from the buffer and empties the buffer.
func (b *Buffer) takeAll() []*PendingMessage {
	b.Lock()
	defer b.Unlock()

	toSend := b.pending[:]
	b.pending = nil

	return toSend
}
