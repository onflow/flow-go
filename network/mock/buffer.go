package mock

import "sync"

type Message struct {
	From      string
	EngineID  uint8
	Event     interface{}
	TargetIDs []string
}

// Buffer buffers all the messages to be sent over the mock network from one node to another node
type Buffer struct {
	sync.Mutex
	pending []*Message
}

func NewBuffer() *Buffer {
	return &Buffer{
		pending: make([]*Message, 0),
	}
}

func (b *Buffer) Save(from string, engineID uint8, event interface{}, targetIDs []string) {
	b.Lock()
	defer b.Unlock()
	b.pending = append(b.pending, &Message{
		From:      from,
		EngineID:  engineID,
		Event:     event,
		TargetIDs: targetIDs,
	})
}

// Flush recursively delivers the pending messages until no pending message exist
func (b *Buffer) Flush(sendOne func(*Message) error) {
	for {
		toSend := b.prepare()

		if len(toSend) == 0 {
			return
		}

		for _, msg := range toSend {
			sendOne(msg)
		}
	}
}

func (b *Buffer) prepare() []*Message {
	b.Lock()
	defer b.Unlock()

	toSend := make([]*Message, 0)

	for _, msg := range b.pending {
		toSend = append(toSend, msg)
	}

	b.pending = make([]*Message, 0)
	return toSend
}
