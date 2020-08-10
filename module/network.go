// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/network"
)

// Network represents the network layer of the node. It allows processes that
// work across the peer-to-peer network to register themselves as an engine with
// a unique engine ID. The returned conduit allows the process to communicate to
// the same engine on other nodes across the network in a network-agnostic way.
type Network interface {

	// Register will make the network aware of a new process in the business logic
	// of the node that wants to communicate across the network. It will return a
	// conduit which connects the process to a bus allowing it to communicate with
	// all other engines on the network that are on the same channel.
	Register(channelID uint8, engine network.Engine) (network.Conduit, error)
}
