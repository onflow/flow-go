package network

import (
	"fmt"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
)

type ProtocolType string

func (m ProtocolType) String() string {
	return string(m)
}

const (
	// ProtocolTypeUnicast is protocol type for unicast messages.
	ProtocolTypeUnicast ProtocolType = "unicast"

	// ProtocolTypePubSub is the protocol type for pubsub messages.
	ProtocolTypePubSub ProtocolType = "pubsub"
)

// IncomingMessageScope captures the context around an incoming message.
type IncomingMessageScope struct {
	originId       flow.Identifier  // the origin node ID.
	eventId        hash.Hash        // hash of the payload and channel.
	msg            *message.Message // the message received.
	decodedPayload interface{}      // decoded payload of the message.
	protocol       ProtocolType     // the type of protocol used to receive the message.
}

func NewIncomingScope(originId flow.Identifier, protocol ProtocolType, msg *message.Message, decodedPayload interface{}) (*IncomingMessageScope, error) {
	eventId, err := p2p.EventId(channels.Channel(msg.ChannelID), msg.Payload)
	if err != nil {
		return nil, fmt.Errorf("could not compute event id: %w", err)
	}
	return &IncomingMessageScope{
		eventId:        eventId,
		originId:       originId,
		msg:            msg,
		decodedPayload: decodedPayload,
		protocol:       protocol,
	}, nil
}

func (m *IncomingMessageScope) OriginId() flow.Identifier {
	return m.originId
}

func (m *IncomingMessageScope) Message() *message.Message {
	return m.msg
}

func (m *IncomingMessageScope) DecodedPayload() interface{} {
	return m.DecodedPayload
}

func (m *IncomingMessageScope) Protocol() ProtocolType {
	return m.protocol
}

func (m *IncomingMessageScope) Channel() string {
	return m.msg.ChannelID
}

func (m *IncomingMessageScope) Size() int {
	return m.msg.Size()
}

func (m *IncomingMessageScope) TargetIDs() [][]byte {
	return m.msg.TargetIDs
}

func (m *IncomingMessageScope) EventID() []byte {
	return m.eventId[:]
}

func (m *IncomingMessageScope) PayloadType() string {
	return p2p.MessageType(m.msg.Payload)
}
