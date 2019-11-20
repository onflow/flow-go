package gossip

import (
	"context"
)

// Underlay represents the layer that provides the network implementation for establishing
// connections with peers, starting a service for accepting incoming connection request, as well
// as taking callback functions for notifying inbound messages.
type Underlay interface {
	// Handle accepts a callback function which will be called when the server receives a message from
	// a peer.
	Handle(onReceive OnReceiveCallback) error
	// Start starts the server with a given address. The address should contain the port info.
	// For instance: "198.51.100.1:80"
	Start(address string) error
	// Stop stops the server and drops all connections
	Stop() error

	// Dial establishes the connection with a peer specified by the peer's address
	Dial(address string) (Connection, error)
}

// OnReceiveCallback is the callback type for receiving incoming messages from a peer
type OnReceiveCallback func(sender string, msg []byte)

// Connection represents a client connection to a peer through which a node can send messages
// to a peer.
type Connection interface {
	// Send sends a message to the peer through the alive connection.
	Send(ctx context.Context, msg []byte) error
	// OnClosed takes a callback to be notified when the peer closed the connection
	OnClosed(func()) error
	// Close close the connection
	Close() error
}
