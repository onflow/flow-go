package insecure

import (
	"github.com/onflow/flow-go/module/component"
)

// OrchestratorNetwork represents the networking interface that is available to the attack orchestrator for sending messages "through"
// the corrupt network and corrupt nodes "to" the rest of the network.
// This interface is used by attack orchestrators to communicate with the corrupt network.
type OrchestratorNetwork interface {
	component.Component
	// SendEgress is called when the attack orchestrator sends an egress message to another node (corrupt or honest) via the corrupt flow network.
	SendEgress(*EgressEvent) error

	// SendIngress is called when an attack orchestrator allows a message (sent from an honest or corrupt node) to reach a corrupt node.
	// The message could be the originally intended message or another valid message (as necessary for the attack).
	SendIngress(*IngressEvent) error

	// Observe is the inbound message handler of the attack orchestrator network.
	// "Inbound" message means it's coming into the orchestrator network (either from a corrupt node, for an egress message OR
	// from another node on the network (honest or corrupt), for an ingress message).
	// The message that is observed can be an ingress or egress message.
	// An observed egress message is when a corrupt node (that's controlled by an attack orchestrator) sends a message to another node.
	// An observed ingress message is when another node sends a message to a corrupt node that's controlled by the attack orchestrator.
	// Instead of dispatching messages to the networking layer of Flow, the corrupt network
	// dispatches the message to the orchestrator network through a remote call to this method.
	Observe(*Message)
}
