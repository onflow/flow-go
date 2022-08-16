package insecure

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
)

const (
	ProtocolUnicast   = "protocol-unicast"
	ProtocolMulticast = "protocol-multicast"
	ProtocolPublish   = "protocol-publish"
	ProtocolUnknown   = "unknown-protocol"
)

// EgressEvent represents the data model that is exchanged between the attacker and the attack orchestrator.
// An egress event is the protocol-level representation of an outgoing message of a corruptible conduit.
// The corruptible conduit relays the message to the attacker instead of dispatching it through the Flow network.
// The attacker decodes the message into an event and relays it to the orchestrator.
// Each corrupted conduit is uniquely identified by 1) corrupted node ID and 2) channel
type EgressEvent struct {
	CorruptedNodeId flow.Identifier  // identifier of corrupted flow node that this corruptible conduit belongs to
	Channel         channels.Channel // channel of the event on the corrupted conduit
	Protocol        Protocol         // networking-layer protocol that this event was meant to send on.
	TargetNum       uint32           // number of randomly chosen targets (used in multicast protocol).

	// set of target identifiers (can be any subset of nodes, either honest or corrupted).
	TargetIds flow.IdentifierList

	// the protocol-level event that the corrupted node is relaying to
	// the attacker. The event is originated by the corrupted node, and is
	// sent to attacker to decide on its content before dispatching it to the
	// Flow network.
	FlowProtocolEvent interface{}
}

// IngressEvent is the incoming event coming to a corrupted node (from an honest or corrupted node)
type IngressEvent struct {
	OriginID          flow.Identifier
	Channel           channels.Channel
	FlowProtocolEvent interface{}
}

func ProtocolStr(p Protocol) string {
	switch p {
	case Protocol_UNICAST:
		return ProtocolUnicast
	case Protocol_MULTICAST:
		return ProtocolMulticast
	case Protocol_PUBLISH:
		return ProtocolPublish
	}

	return ProtocolUnknown
}
