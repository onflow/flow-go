// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package middleware

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/gossip/libp2p/message"
)

// Middleware represents the middleware layer, which manages the connections to
// our direct neighbours on the network. It handles the creation & teardown of
// connections, as well as reading & writing to/from the connections.
type Middleware interface {
	// Start will start the middleware.
	Start(overlay Overlay) error

	// Stop will end the execution of the middleware and wait for it to end.
	Stop()

	// Send sends the message to the set of target ids
	// If there is only one target NodeID, then a direct 1-1 connection is used by calling middleware.sendDirect
	// Otherwise, middleware.Publish is used, which uses the PubSub method of communication.
	//
	// Deprecated: Send exists for historical compatibility, and should not be used on new
	// developments. It is planned to be cleaned up in near future. Proper utilization of Dispatch or
	// Publish are recommended instead.
	Send(channelID string, msg *message.Message, targetIDs ...flow.Identifier) error

	// Dispatch sends msg on a 1-1 direct connection to the target ID. It models a guaranteed delivery asynchronous
	// direct one-to-one connection on the underlying network. No intermediate node on the overlay is utilized
	// as the router.
	//
	// Dispatch should be used whenever guaranteed delivery to a specific target is required. Otherwise, Publish is
	// a more efficient candidate.
	SendDirect(msg *message.Message, targetID flow.Identifier) error

	// Publish publishes msg on the channel. It models a distributed broadcast where the message is meant for all or
	// a many nodes subscribing to the channel ID. It does not guarantee the delivery though, and operates on a best
	// effort.
	Publish(msg *message.Message, channelID string) error

	// Subscribe will subscribe the middleware for a topic with the fully qualified channel ID name
	Subscribe(channelID string) error

	// Unsubscribe will unsubscribe the middleware for a topic with the fully qualified channel ID name
	Unsubscribe(channelID string) error

	// Ping pings the target node and returns the ping RTT or an error
	Ping(targetID flow.Identifier) (time.Duration, error)

	// UpdateAllowList fetches the most recent identity of the nodes from overlay
	// and updates the underlying libp2p node.
	UpdateAllowList() error
}

// Overlay represents the interface that middleware uses to interact with the
// overlay network layer.
type Overlay interface {
	// Topology returns an identity list of nodes which this node should be directly connected to as peers
	Topology() (flow.IdentityList, error)
	// Identity returns a map of all identifier to flow identity
	Identity() (map[flow.Identifier]flow.Identity, error)
	Receive(nodeID flow.Identifier, msg *message.Message) error
}

// Connection represents an interface to read from & write to a connection.
type Connection interface {
	Send(msg interface{}) error
	Receive() (interface{}, error)
}
