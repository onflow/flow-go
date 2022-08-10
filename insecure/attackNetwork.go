package insecure

import (
	"github.com/onflow/flow-go/module/component"
)

// AttackNetwork represents the networking interface that is available to the attacker for sending messages "through"
// the corrupted network and corrupted nodes "to" the rest of the network.
// This interface is used by attack orchestrators to communicate with the corrupted network.
type AttackNetwork interface {
	component.Component
	// SendEgress is called when the attacker sends a message to another node (corrupted or honest) via the corrupted flow network.
	SendEgress(*EgressEvent) error

	// SendIngress is called when an attacker allows a message (sent from an honest or corrupted node) to reach a corrupted node.
	// The message could be the originally intended message or another valid message (as necessary for the attack).
	SendIngress(*IngressEvent) error

	// Observe is the inbound message handler of the attack network (NOT the ingress message to a corrupted node).
	// The message that is observed can be an ingress or egress message.
	// An observed egress message is when a corrupted node (that's controlled by an attacker) sends a message to another node.
	// An observed ingress message is when another node is sending a message to a corrupted node that's controlled by the attacker.
	// Instead of dispatching messages to the networking layer of Flow, the corrupted network
	// dispatches the outgoing message to the attack network through a remote call to this method.
	Observe(*Message)
}
