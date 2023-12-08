package message

import (
	"fmt"
	"strings"

	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
)

const (
	// eventIDPackingPrefix is used as a salt to generate payload hash for messages.
	eventIDPackingPrefix = "libp2ppacking"
)

// EventId computes the event ID for a given channel and payload (i.e., the hash of the payload and channel).
// All errors returned by this function are benign and should not cause the node to crash.
// It errors if the hash function fails to hash the payload and channel.
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

// MessageType returns the type of the message payload.
func MessageType(decodedPayload interface{}) string {
	return strings.TrimLeft(fmt.Sprintf("%T", decodedPayload), "*")
}

// IncomingMessageScope captures the context around an incoming message that is received by the network layer.
type IncomingMessageScope struct {
	originId       flow.Identifier     // the origin node ID.
	targetIds      flow.IdentifierList // the target node IDs (i.e., intended recipients).
	eventId        hash.Hash           // hash of the payload and channel.
	msg            *Message            // the raw message received.
	decodedPayload interface{}         // decoded payload of the message.
	protocol       ProtocolType        // the type of protocol used to receive the message.
}

// NewIncomingScope creates a new incoming message scope.
// All errors returned by this function are benign and should not cause the node to crash, especially that it is not
// safe to crash the node when receiving a message.
// It errors if event id (i.e., hash of the payload and channel) cannot be computed, or if it fails to
// convert the target IDs from bytes slice to a flow.IdentifierList.
func NewIncomingScope(originId flow.Identifier, protocol ProtocolType, msg *Message, decodedPayload interface{}) (*IncomingMessageScope, error) {
	eventId, err := EventId(channels.Channel(msg.ChannelID), msg.Payload)
	if err != nil {
		return nil, fmt.Errorf("could not compute event id: %w", err)
	}

	targetIds, err := flow.ByteSlicesToIds(msg.TargetIDs)
	if err != nil {
		return nil, fmt.Errorf("could not convert target ids: %w", err)
	}
	return &IncomingMessageScope{
		eventId:        eventId,
		originId:       originId,
		msg:            msg,
		decodedPayload: decodedPayload,
		protocol:       protocol,
		targetIds:      targetIds,
	}, nil
}

func (m IncomingMessageScope) OriginId() flow.Identifier {
	return m.originId
}

func (m IncomingMessageScope) Proto() *Message {
	return m.msg
}

func (m IncomingMessageScope) DecodedPayload() interface{} {
	return m.decodedPayload
}

func (m IncomingMessageScope) Protocol() ProtocolType {
	return m.protocol
}

func (m IncomingMessageScope) Channel() channels.Channel {
	return channels.Channel(m.msg.ChannelID)
}

func (m IncomingMessageScope) Size() int {
	return m.msg.Size()
}

func (m IncomingMessageScope) TargetIDs() flow.IdentifierList {
	return m.targetIds
}

func (m IncomingMessageScope) EventID() []byte {
	return m.eventId[:]
}

func (m IncomingMessageScope) PayloadType() string {
	return MessageType(m.decodedPayload)
}

// OutgoingMessageScope captures the context around an outgoing message that is about to be sent.
type OutgoingMessageScope struct {
	targetIds flow.IdentifierList               // the target node IDs.
	topic     channels.Topic                    // the topic, i.e., channel-id/spork-id.
	payload   interface{}                       // the payload to be sent.
	encoder   func(interface{}) ([]byte, error) // the encoder to encode the payload.
	msg       *Message                          // raw proto message sent on wire.
	protocol  ProtocolType                      // the type of protocol used to send the message.
}

// NewOutgoingScope creates a new outgoing message scope.
// All errors returned by this function are benign and should not cause the node to crash.
// It errors if the encoder fails to encode the payload into a protobuf message, or
// if the number of target IDs does not match the protocol type (i.e., unicast messages
// should have exactly one target ID, while pubsub messages should have at least one target ID).
func NewOutgoingScope(
	targetIds flow.IdentifierList,
	topic channels.Topic,
	payload interface{},
	encoder func(interface{}) ([]byte, error),
	protocolType ProtocolType) (*OutgoingMessageScope, error) {
	scope := &OutgoingMessageScope{
		targetIds: targetIds,
		topic:     topic,
		payload:   payload,
		encoder:   encoder,
		protocol:  protocolType,
	}

	if protocolType == ProtocolTypeUnicast {
		// for unicast messages, we should have exactly one target.
		if len(targetIds) != 1 {
			return nil, fmt.Errorf("expected exactly one target id for unicast message, got: %d", len(targetIds))
		}
	}
	if protocolType == ProtocolTypePubSub {
		// for pubsub messages, we should have at least one target.
		if len(targetIds) == 0 {
			return nil, fmt.Errorf("expected at least one target id for pubsub message, got: %d", len(targetIds))
		}
	}

	msg, err := scope.buildMessage()
	if err != nil {
		return nil, fmt.Errorf("could not build message: %w", err)
	}
	scope.msg = msg
	return scope, nil
}

func (o OutgoingMessageScope) TargetIds() flow.IdentifierList {
	return o.targetIds
}

func (o OutgoingMessageScope) Size() int {
	return o.msg.Size()
}

func (o OutgoingMessageScope) PayloadType() string {
	return MessageType(o.payload)
}

func (o OutgoingMessageScope) Topic() channels.Topic {
	return o.topic
}

// buildMessage builds the raw proto message to be sent on the wire.
func (o OutgoingMessageScope) buildMessage() (*Message, error) {
	payload, err := o.encoder(o.payload)
	if err != nil {
		return nil, fmt.Errorf("could not encode payload: %w", err)
	}

	emTargets := make([][]byte, 0)
	for _, targetId := range o.targetIds {
		tempID := targetId // avoid capturing loop variable
		emTargets = append(emTargets, tempID[:])
	}

	channel, ok := channels.ChannelFromTopic(o.topic)
	if !ok {
		return nil, fmt.Errorf("could not convert topic to channel: %s", o.topic)
	}

	return &Message{
		TargetIDs: emTargets,
		ChannelID: channel.String(),
		Payload:   payload,
	}, nil
}

func (o OutgoingMessageScope) Proto() *Message {
	return o.msg
}
