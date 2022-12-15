package network

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
)

const (
	// eventIDPackingPrefix is used as a salt to generate payload hash for messages.
	eventIDPackingPrefix = "libp2ppacking"
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
	eventId, err := EventId(channels.Channel(msg.ChannelID), msg.Payload)
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

func (m IncomingMessageScope) OriginId() flow.Identifier {
	return m.originId
}

func (m IncomingMessageScope) Message() *message.Message {
	return m.msg
}

func (m IncomingMessageScope) DecodedPayload() interface{} {
	return m.decodedPayload
}

func (m IncomingMessageScope) Protocol() ProtocolType {
	return m.protocol
}

func (m IncomingMessageScope) Channel() string {
	return m.msg.ChannelID
}

func (m IncomingMessageScope) Size() int {
	return m.msg.Size()
}

func (m IncomingMessageScope) TargetIDs() [][]byte {
	return m.msg.TargetIDs
}

func (m IncomingMessageScope) EventID() []byte {
	return m.eventId[:]
}

func (m IncomingMessageScope) PayloadType() string {
	return MessageType(m.msg.Payload)
}

type OutgoingMessageScope struct {
	targetId  flow.Identifier                   // the target node IDs.
	channelId string                            // the channel ID.
	payload   interface{}                       // the payload to be sent.
	encoder   func(interface{}) ([]byte, error) // the encoder to encode the payload.
	msg       *message.Message                  // proto message sent on wire.
	protocol  ProtocolType                      // the type of protocol used to send the message.
}

func NewOutgoingScope(
	targetId flow.Identifier,
	channelId string,
	payload interface{},
	encoder func(interface{}) ([]byte, error),
	protocolType ProtocolType) (*OutgoingMessageScope, error) {
	scope := &OutgoingMessageScope{
		targetId:  targetId,
		channelId: channelId,
		payload:   payload,
		encoder:   encoder,
		protocol:  protocolType,
	}
	msg, err := scope.buildMessage()
	if err != nil {
		return nil, fmt.Errorf("could not build message: %v", err)
	}
	scope.msg = msg
	return scope, nil
}

func (o OutgoingMessageScope) TargetId() flow.Identifier {
	return o.targetId
}

func (o OutgoingMessageScope) Size() int {
	return o.msg.Size()
}

func (o OutgoingMessageScope) MessageType() string {
	return MessageType(o.payload)
}

func (o OutgoingMessageScope) Channel() string {
	return o.channelId
}

func (o OutgoingMessageScope) buildMessage() (*message.Message, error) {
	payload, err := o.encoder(o.payload)
	if err != nil {
		return nil, fmt.Errorf("could not encode payload: %w", err)
	}

	//var emTargets [][]byte = [][]byte{o.targetId[:]}
	//for _, targetId := range o.targetIds {
	//	tempID := targetId // avoid capturing loop variable
	//	emTargets = append(emTargets, tempID[:])
	//}

	return &message.Message{
		TargetIDs: [][]byte{o.targetId[:]},
		ChannelID: o.channelId,
		Payload:   payload,
	}, nil
}

func (o OutgoingMessageScope) Proto() *message.Message {
	return o.msg
}

func EventId(channel channels.Channel, payload []byte) (hash.Hash, error) {
	// use a hash with an engine-specific salt to get the payload hash
	h := hash.NewSHA3_384()
	_, err := h.Write([]byte(eventIDPackingPrefix + channel))
	if err != nil {
		return nil, fmt.Errorf("could not hash channel as salt: %w", err)
	}

	_, err = h.Write(payload)
	if err != nil {
		return nil, fmt.Errorf("could not hash event: %w", err)
	}

	return h.SumHash(), nil
}

func MessageType(decodedPayload interface{}) string {
	return strings.TrimLeft(fmt.Sprintf("%T", decodedPayload), "*")
}
