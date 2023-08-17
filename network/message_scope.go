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

// IncomingMessageScope defines the interface for handling incoming message scope.
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

// OutgoingMessageScope defines the interface for handling outgoing message scope.
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
