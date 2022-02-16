package network

import (
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/onflow/flow-go/model/flow"
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
	Register(channel Channel, messageProcessor MessageProcessor) (Conduit, error)

	// RegisterBlobService registers a BlobService on the given channel, using the given datastore to retrieve values.
	// The returned BlobService can be used to request blocks from the network.
	// TODO: We should return a function that can be called to unregister / close the BlobService
	RegisterBlobService(channel Channel, store datastore.Batching, opts ...BlobServiceOption) (BlobService, error)

	// RegisterPingService registers a ping protocol handler for the given protocol ID
	RegisterPingService(pingProtocolID protocol.ID, pingInfoProvider PingInfoProvider) (PingService, error)
}

// Adapter is a wrapper around the Network implementation. It only exposes message dissemination functionalities.
// Adapter is meant to be utilized by the Conduit interface to send messages to the Network layer to be
// delivered to the remote targets.
type Adapter interface {
	// UnicastOnChannel sends the message in a reliable way to the given recipient.
	UnicastOnChannel(Channel, interface{}, flow.Identifier) error

	// PublishOnChannel sends the message in an unreliable way to all the given recipients.
	PublishOnChannel(Channel, interface{}, ...flow.Identifier) error

	// MulticastOnChannel unreliably sends the specified event over the channel to randomly selected number of recipients
	// selected from the specified targetIDs.
	MulticastOnChannel(Channel, interface{}, uint, ...flow.Identifier) error

	// UnRegisterChannel unregisters the engine for the specified channel. The engine will no longer be able to send or
	// receive messages from that channel.
	UnRegisterChannel(channel Channel) error
}
