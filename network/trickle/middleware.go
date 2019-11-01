// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package trickle

// Middleware represents the middleware layer, which manages the connections to
// our direct neighbours on the network. It handles the creation & teardown of
// connections, as well as reading & writing to/from the connections.
type Middleware interface {
	Start(overlay Overlay)
	Stop()
	Send(connID string, msg interface{}) error
}

// Overlay represents the interface that middleware uses to interact with the
// overlay network layer.
type Overlay interface {
	Address() (string, error)
	Handshake(conn Connection) (string, error)
	Receive(connID string, msg interface{}) error
	Cleanup(connID string) error
}

// Connection represents an interface to read from & write to a connection.
type Connection interface {
	Send(msg interface{}) error
	Receive() (interface{}, error)
}
