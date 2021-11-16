package network

import (
	"github.com/ipfs/go-datastore"

	"github.com/onflow/flow-go/module/component"
)

// Network represents the network layer of the node. It allows processes that
// work across the peer-to-peer network to register themselves as an engine with
// a unique engine ID. The returned conduit allows the process to communicate to
// the same engine on other nodes across the network in a network-agnostic way.
type Network interface {
	component.Component
	// Register will subscribe to the channel with the given engine and
	// the engine will be notified with incoming messages on the channel.
	// The returned Conduit can be used to send messages to engines on other nodes subscribed to the same channel
	// On a single node, only one engine can be subscribed to a channel at any given time.
	Register(channel Channel, engine Engine) (Conduit, error)

	// RegisterBlobService registers a BlobService on the given channel, using the given datastore to retrieve values.
	// The returned BlobService can be used to request blocks from the network.
	RegisterBlobService(channel Channel, store datastore.Batching) (BlobService, error)
}
