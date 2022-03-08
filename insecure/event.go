package insecure

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// Event represents the data model that is exchanged between the attacker and the attack orchestrator.
// An event is the protocol-level representation of an outgoing message of a corruptible conduit.
// The corruptible conduit relays the message to the attacker instead of dispatching it through the Flow network.
// The attacker decodes the message into an event and relays it to the orchestrator.
type Event struct {
	CorruptedId flow.Identifier     // identifier of corrupted conduit
	Channel     network.Channel     // channel of the event on the corrupted conduit
	Content     interface{}         // the protocol-level event itself.
	Protocol    Protocol            // networking-layer protocol that this event was meant to send on.
	TargetNum   uint32              // number of randomly chosen targets (used in multicast protocol).
	TargetIds   flow.IdentifierList // set of target identifiers
}
