// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package libp2p

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Middleware represents the middleware layer, which manages the connections to
// our direct neighbours on the network. It handles the creation & teardown of
// connections, as well as reading & writing to/from the connections.
type Middleware interface {
	Start(overlay Overlay)
	Stop()
	Send(nodeID flow.Identifier, msg interface{}) error
}

// Overlay represents the interface that middleware uses to interact with the
// overlay network layer.
type Overlay interface {
	Identity() (flow.Identity, error)
	Handshake(conn Connection) (flow.Identifier, error)
	Receive(nodeID flow.Identifier, msg interface{}) error
	Cleanup(nodeID flow.Identifier) error
}

// Connection represents an interface to read from & write to a connection.
type Connection interface {
	Send(msg interface{}) error
	Receive() (interface{}, error)
}
