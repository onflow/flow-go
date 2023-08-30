// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package network

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/network/channels"
)

// Middleware represents the middleware layer, which manages the connections to
// our direct neighbours on the network. It handles the creation & teardown of
// connections, as well as reading & writing to/from the connections.
type Middleware interface {
	component.Component
	DisallowListNotificationConsumer

	// Subscribe subscribes the middleware to a channel.
	// No errors are expected during normal operation.
	Subscribe(channel channels.Channel) error

	// Unsubscribe unsubscribes the middleware from a channel.
	// All errors returned from this function can be considered benign.
	Unsubscribe(channel channels.Channel) error

	// UpdateNodeAddresses fetches and updates the addresses of all the authorized participants
	// in the Flow protocol.
	UpdateNodeAddresses()
}

// Overlay represents the interface that middleware uses to interact with the
// overlay network layer.
type Overlay interface {
	// Topology returns an identity list of nodes which this node should be directly connected to as peers
	Topology() flow.IdentityList

	// Identities returns a list of all Flow identities on the network
	Identities() flow.IdentityList

	// Identity returns the Identity associated with the given peer ID, if it exists
	Identity(peer.ID) (*flow.Identity, bool)

	Receive(IncomingMessageScope) error
}

// Connection represents an interface to read from & write to a connection.
type Connection interface {
	Send(msg interface{}) error
	Receive() (interface{}, error)
}
