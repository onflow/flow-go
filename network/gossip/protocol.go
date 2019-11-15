package gossip

import (
	"context"

	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

// ProtocolMessage represents the data received from a peer or to be delivered to a peer
type ProtocolMessage struct {
	// Message is the message data
	Message messages.GossipMessage
	// Broadcast indicate whether the receiver will need to broadcast it
	Broadcast bool
}

// Protocol represents the protocol layer that provides the network implementation for establishing
// connections with peers, starting a service for accepting incoming connection request, as well
// as taking callback functions for notifying inbound messages.
type Protocol interface {
	// Handle accepts a callback function which will be called when the server receives a message from
	// a peer.
	Handle(onReceive func(sender string, msg ProtocolMessage)) error
	// Start starts the server with a given address and port
	Start(address string, port string) error
	// Stop stops the server and drops all connections
	Stop() error

	// Dial establishes the connection with a peer specified by the peer's address
	Dial(address string) (Connection, error)
}

// Connection represents a client connection to a peer through which a node can send messages
// to a peer.
type Connection interface {
	// Send sends a message to the peer through the alive connection.
	Send(ctx context.Context, msg *ProtocolMessage) error
	// onClosed takes a callback to be notified when the peer closed the connection
	OnClosed(func()) error
	// Close close the connection
	Close() error
}
