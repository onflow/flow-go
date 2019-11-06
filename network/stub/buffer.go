package stub

import (
	"sync"
)

// PendingMessage is a pending message to be sent
type PendingMessage struct {
	// The sender node id
	From     string
	EngineID uint8
	Event    interface{}
	// The id of the receiver nodes
	TargetIDs []string
}

// IsOneToAll returns whether a pending message is supposed to be sent to all other nodes
// in the network
func (p *PendingMessage) IsOneToAll() bool {
	return len(p.TargetIDs) == 0
}

// IsOneToOne returns whether a pending message is supposed to be sent to another
func (p *PendingMessage) IsOneToOne() bool {
	return len(p.TargetIDs) == 1
}

// IsOneToMany returns whether a pending message is supposed to be sent to multiple nodes
func (p *PendingMessage) IsOneToMany() bool {
	return len(p.TargetIDs) > 1
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
func (b *Buffer) Save(from string, engineID uint8, event interface{}, targetIDs []string) {
	b.Lock()
	defer b.Unlock()
	b.pending = append(b.pending, &PendingMessage{
		From:      from,
		EngineID:  engineID,
		Event:     event,
		TargetIDs: targetIDs,
	})
}

// Flush delivers the pending messages in the buffer
func (b *Buffer) Flush(sendOne func(*PendingMessage) error) {
	toSend := b.takeAll()

	for _, msg := range toSend {
		sendOne(msg)
	}
}

// popAll takes all pending messages from the buffer and empty the buffer.
func (b *Buffer) takeAll() []*PendingMessage {
	b.Lock()
	defer b.Unlock()

	toSend := b.pending[:]
	b.pending = nil

	return toSend
}
