package network

// MessageValidator validates the incoming message. Message validation happens in the network right before it is
// delivered to the network.
type MessageValidator interface {
	// Validate validates the message and returns true if the message is to be retained and false if it needs to be dropped
	Validate(msg IncomingMessageScope) bool
}
