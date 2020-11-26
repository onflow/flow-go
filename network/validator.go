package network

import "github.com/onflow/flow-go/network/message"

// MessageValidator validates the incoming message.
type MessageValidator interface {
	// Validate validates the message and returns true if the message is to be retained and false if it needs to be dropped
	Validate(msg message.Message) bool
}
