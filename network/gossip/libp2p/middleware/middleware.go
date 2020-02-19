// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package middleware

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Middleware represents the middleware layer, which manages the connections to
// our direct neighbours on the network. It handles the creation & teardown of
// connections, as well as reading & writing to/from the connections.
type Middleware interface {
	Start(overlay Overlay) error
	Stop()
	Send(nodeID flow.Identifier, msg interface{}) error
	Publish(topic string, msg interface{}) error
	Subscribe(channelID uint8) error
}

// Overlay represents the interface that middleware uses to interact with the
// overlay network layer.
type Overlay interface {
	// Topology returns the identities of a uniform subset of nodes in protocol state
	Topology() (map[flow.Identifier]flow.Identity, error)
	// Identity returns a map of all identifier to flow identity
	Identity() (map[flow.Identifier]flow.Identity, error)
	Receive(nodeID flow.Identifier, msg interface{}) error
}

// Connection represents an interface to read from & write to a connection.
type Connection interface {
	Send(msg interface{}) error
	Receive() (interface{}, error)
}
