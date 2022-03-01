package insecure

import (
	"github.com/onflow/flow-go/model/flow"
)

// AttackVector represents the logic of an attack that is conducted by the adversary.
type AttackVector interface {
	// Handle implements the event handler of a certain attack upon receiving a message from a corrupted node.
	// As the result of processing the message, it may send some messages "through" the corrupted nodes "to" the
	// rest of the network.
	Handle(interface{}, flow.IdentityList, AttackNetwork) error
}
