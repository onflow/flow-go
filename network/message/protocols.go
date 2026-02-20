package message

import "slices"

const (
	// ProtocolTypeUnicast is protocol type for unicast messages.
	ProtocolTypeUnicast ProtocolType = "unicast"
	// ProtocolTypePubSub is the protocol type for pubsub messages.
	ProtocolTypePubSub ProtocolType = "pubsub"
)

// ProtocolType defines the type of the protocol a message is sent over. Currently, we have two types of protocols:
// - unicast: a message is sent to a single node through a direct connection.
// - pubsub: a message is sent to one or more nodes through a pubsub channel.
type ProtocolType string

func (m ProtocolType) String() string {
	return string(m)
}

type Protocols []ProtocolType

// Contains returns true if the protocol is in the list of Protocols.
func (pr Protocols) Contains(protocol ProtocolType) bool {
	return slices.Contains(pr, protocol)
}
