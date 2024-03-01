package queue

import (
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"strconv"
	"time"
)

// MessageEntity is a data structure for storing messages in HeroQueue.
type MessageEntity struct {
	Msg engine.Message
	id  flow.Identifier
}

var _ flow.Entity = (*MessageEntity)(nil)

// NewMessageEntity returns a new message entity.
func NewMessageEntity(msg *engine.Message) MessageEntity {
	id := identifierOfMessage(msg)
	return MessageEntity{
		Msg: *msg,
		id:  id,
	}
}

// NewMessageEntityWithNonce creates a new message entity adding a nonce to the id calculation.
// This prevents de-duplication of otherwise identical messages stored in the queue. 
func NewMessageEntityWithNonce(msg *engine.Message) MessageEntity {
	id := identifierOfMessage(struct {
		*engine.Message
		Nonce string
	}{
		msg,
		uint64(time.Now().UnixNano()),
	})
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

func identifierOfMessage(msg interface{}) flow.Identifier {
	return flow.MakeID(msg)
}
