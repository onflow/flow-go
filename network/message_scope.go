package network

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
)

// IncomingMessageScope defines the interface for incoming message scopes, i.e., self-contained messages that have been
// received on the wire and are ready to be processed.
type IncomingMessageScope interface {
	// OriginId returns the origin node ID.
	OriginId() flow.Identifier

	// Proto returns the raw message received.
	Proto() *message.Message

	// DecodedPayload returns the decoded payload of the message.
	DecodedPayload() interface{}

	// Protocol returns the type of protocol used to receive the message.
	Protocol() message.ProtocolType

	// Channel returns the channel of the message.
	Channel() channels.Channel

	// Size returns the size of the message.
	Size() int

	// TargetIDs returns the target node IDs, i.e., the intended recipients.
	TargetIDs() flow.IdentifierList

	// EventID returns the hash of the payload and channel.
	EventID() []byte

	// PayloadType returns the type of the decoded payload.
	PayloadType() string
}

// OutgoingMessageScope defines the interface for building outgoing message scopes, i.e., self-contained messages
// that are ready to be sent on the wire.
type OutgoingMessageScope interface {
	// TargetIds returns the target node IDs.
	TargetIds() flow.IdentifierList

	// Size returns the size of the message.
	Size() int

	// PayloadType returns the type of the payload to be sent.
	PayloadType() string

	// Topic returns the topic, i.e., channel-id/spork-id.
	Topic() channels.Topic

	// Proto returns the raw proto message sent on the wire.
	Proto() *message.Message
}
