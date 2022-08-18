package insecure

import (
	"github.com/onflow/flow-go/module/component"
)

// AttackNetwork represents the networking interface that is available to the attacker for sending messages "through" corrupted nodes
// "to" the rest of the network.
type AttackNetwork interface {
	component.Component
	// Send enforces dissemination of given event via its encapsulated corrupted node networking layer through the Flow network.
	Send(*EgressEvent) error
	// Observe is the inbound message handler of the attack network.
	// Instead of dispatching their messages to the networking layer of Flow, the conduits of corrupted nodes
	// dispatch the outgoing message to the attack network through a remote call to this method.
	Observe(*Message)
}
