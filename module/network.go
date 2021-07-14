// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/onflow/flow-go/network"
)

// Network represents the network layer of the node. It allows processes that
// work across the peer-to-peer network to register themselves as a message
// processor with a unique processor ID. The returned conduit allows the process
// to communicate to the same processor on other nodes across the network in a
// network-agnostic way.
type Network interface {

	// Register will subscribe to the channel with the given message processor and
	// the message processor will be notified with incoming messages on the channel.
	// The returned Conduit can be used to send messages to message processors on other nodes
	// subscribed to the same channel
	// On a single node, only one message processor can be subscribed to a channel at any given time.
	Register(channel network.Channel, messageProcessor network.MessageProcessor) (network.Conduit, error)
}
