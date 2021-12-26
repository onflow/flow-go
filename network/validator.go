package network

import (
	"github.com/onflow/flow-go/model/flow"
)

// MessageValidator validates the incoming message. Message validation happens in the middleware right before it is
// delivered to the network.
type MessageValidator interface {
	// Validate validates the message and returns true if the message is to be retained and false if it needs to be dropped
	Validate(channel Channel, payload interface{}, origin flow.Identifier) bool
}
