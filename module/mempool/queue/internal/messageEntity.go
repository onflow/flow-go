package internal

import (
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
)

// MessageEntity is an internal data structure for storing messages in HeroQueue.
type MessageEntity struct {
	Msg engine.Message
	id  flow.Identifier
}

var _ flow.Entity = (*MessageEntity)(nil)

func NewMessageEntity(msg *engine.Message) MessageEntity {
	id := identifierOfMessage(msg)
	return MessageEntity{
		Msg: *msg,
		id:  id,
	}
}

func (m MessageEntity) ID() flow.Identifier {
	return m.id
}

func (m MessageEntity) Checksum() flow.Identifier {
	return m.id
}

func identifierOfMessage(msg *engine.Message) flow.Identifier {
	return flow.MakeID(msg)
}
