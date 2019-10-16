// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package overlay

// Middleware represents the middleware layer, which manages the connections to
// our direct neighbours on the network. It handles the creation & teardown of
// connections, as well as reading & writing to/from the connections.
type Middleware interface {
	GetAddress(AddressCallback)

	RunHandshake(HandshakeCallback)
	RunCleanup(CleanupCallback)

	Send(id string, msg interface{}) error
	OnReceive(ReceiveCallback)
}

// AddressCallback is a callback that provides the next address the middleware
// layer should connect to.
type AddressCallback func() (string, error)

// ReceiveCallback is a callback that handles the given event, which originated
// with the node of the given origin (which we are not necessarily directly
// connected to).
type ReceiveCallback func(origin string, msg interface{}) error

// HandshakeCallback is a callback that handles the execution of a handshake on
// the new connection and attributes it a unique node ID.
type HandshakeCallback func(conn Connection) (string, error)

// CleanupCallback is a callback that runs cleanup operations on higher layers
// once the connection with the given node ID is dropped.
type CleanupCallback func(id string) error

// Conn represents an interface to send/receive messages to a specific peer.
type Connection interface {
	Send(msg interface{}) error
	Receive() (interface{}, error)
}
