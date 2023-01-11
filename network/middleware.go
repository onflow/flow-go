// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package network

import (
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/network/channels"
)

// Middleware represents the middleware layer, which manages the connections to
// our direct neighbours on the network. It handles the creation & teardown of
// connections, as well as reading & writing to/from the connections.
type Middleware interface {
	component.Component

	// SetOverlay sets the overlay used by the middleware. This must be called before the middleware can be Started.
	SetOverlay(Overlay)

	// SendDirect sends msg on a 1-1 direct connection to the target ID. It models a guaranteed delivery asynchronous
	// direct one-to-one connection on the underlying network. No intermediate node on the overlay is utilized
	// as the router.
	//
	// Dispatch should be used whenever guaranteed delivery to a specific target is required. Otherwise, Publish is
	// a more efficient candidate.
	// All errors returned from this function can be considered benign.
	SendDirect(msg *OutgoingMessageScope) error

	// Publish publishes a message on the channel. It models a distributed broadcast where the message is meant for all or
	// a many nodes subscribing to the channel. It does not guarantee the delivery though, and operates on a best
	// effort.
	// All errors returned from this function can be considered benign.
	Publish(msg *OutgoingMessageScope) error

	// Subscribe subscribes the middleware to a channel.
	// No errors are expected during normal operation.
	Subscribe(channel channels.Channel) error

	// Unsubscribe unsubscribes the middleware from a channel.
	// All errors returned from this function can be considered benign.
	Unsubscribe(channel channels.Channel) error

	// UpdateNodeAddresses fetches and updates the addresses of all the authorized participants
	// in the Flow protocol.
	UpdateNodeAddresses()

	// NewBlobService creates a new BlobService for the given channel.
	NewBlobService(channel channels.Channel, store datastore.Batching, opts ...BlobServiceOption) BlobService

	// NewPingService creates a new PingService for the given ping protocol ID.
	NewPingService(pingProtocol protocol.ID, provider PingInfoProvider) PingService

	IsConnected(nodeID flow.Identifier) (bool, error)
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

	Receive(*IncomingMessageScope) error
}

// Connection represents an interface to read from & write to a connection.
type Connection interface {
	Send(msg interface{}) error
	Receive() (interface{}, error)
}
