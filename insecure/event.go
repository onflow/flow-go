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
// An egress event is the protocol-level representation of an outgoing message of a corrupt conduit (of a corrupt node).
// The corrupt conduit relays the message to the attacker instead of dispatching it through the Flow network.
// The attacker decodes the message into an event and relays it to the orchestrator.
// Each corrupt conduit is uniquely identified by 1) corrupt node ID and 2) channel
type EgressEvent struct {
	CorruptOriginId flow.Identifier  // identifier of corrupt flow node that this corrupt conduit belongs to
	Channel         channels.Channel // channel of the event on the corrupt conduit
	Protocol        Protocol         // networking-layer protocol that this event was meant to send on.
	TargetNum       uint32           // number of randomly chosen targets (used in multicast protocol).

	// set of target identifiers (can be any subset of nodes, either honest or corrupt).
	TargetIds flow.IdentifierList

	// the protocol-level event that the corrupt node is relaying to
	// the attacker. The event is originated by the corrupt node, and is
	// sent to attacker to decide on its content before dispatching it to the
	// Flow network.
	FlowProtocolEvent interface{}

	// FlowProtocolEventID the flow identifier generated from the hash of the payload of the FlowProtocolEvent.
	// This can be used to map events sent and received by corrupted nodes to further control ingress / egress traffic.
	FlowProtocolEventID flow.Identifier
}

// IngressEvent is the incoming event coming to a corrupt node (from an honest or corrupt node)
type IngressEvent struct {
	OriginID        flow.Identifier
	CorruptTargetID flow.Identifier // corrupt node Id
	Channel         channels.Channel

	FlowProtocolEvent interface{}
	// FlowProtocolEventID the flow identifier generated from the hash of the payload of the FlowProtocolEvent.
	// This can be used to map events sent and received by corrupted nodes to further control ingress / egress traffic.
	FlowProtocolEventID flow.Identifier
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
