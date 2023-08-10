package network

import (
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/network/channels"
)

// NetworkingType is the type of the Flow networking layer. It is used to differentiate between the public (i.e., unstaked)
// and private (i.e., staked) networks.
type NetworkingType uint8

func (t NetworkingType) String() string {
	switch t {
	case PrivateNetwork:
		return "private"
	case PublicNetwork:
		return "public"
	default:
		return "unknown"
	}
}

const (
	// PrivateNetwork indicates that the staked private-side of the Flow blockchain that nodes can only join and leave
	// with a staking requirement.
	PrivateNetwork NetworkingType = iota + 1
	// PublicNetwork indicates that the unstaked public-side of the Flow blockchain that nodes can join and leave at will
	// with no staking requirement.
	PublicNetwork
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
	Register(channel channels.Channel, messageProcessor MessageProcessor) (Conduit, error)

	// RegisterBlobService registers a BlobService on the given channel, using the given datastore to retrieve values.
	// The returned BlobService can be used to request blocks from the network.
	// TODO: We should return a function that can be called to unregister / close the BlobService
	RegisterBlobService(channel channels.Channel, store datastore.Batching, opts ...BlobServiceOption) (BlobService, error)

	// RegisterPingService registers a ping protocol handler for the given protocol ID
	RegisterPingService(pingProtocolID protocol.ID, pingInfoProvider PingInfoProvider) (PingService, error)
}

// Adapter is a wrapper around the Network implementation. It only exposes message dissemination functionalities.
// Adapter is meant to be utilized by the Conduit interface to send messages to the Network layer to be
// delivered to the remote targets.
type Adapter interface {
	MisbehaviorReportConsumer
	// UnicastOnChannel sends the message in a reliable way to the given recipient.
	UnicastOnChannel(channels.Channel, interface{}, flow.Identifier) error

	// PublishOnChannel sends the message in an unreliable way to all the given recipients.
	PublishOnChannel(channels.Channel, interface{}, ...flow.Identifier) error

	// MulticastOnChannel unreliably sends the specified event over the channel to randomly selected number of recipients
	// selected from the specified targetIDs.
	MulticastOnChannel(channels.Channel, interface{}, uint, ...flow.Identifier) error

	// UnRegisterChannel unregisters the engine for the specified channel. The engine will no longer be able to send or
	// receive messages from that channel.
	UnRegisterChannel(channel channels.Channel) error
}

// MisbehaviorReportConsumer set of funcs used to handle MisbehaviorReport disseminated from misbehavior reporters.
type MisbehaviorReportConsumer interface {
	// ReportMisbehaviorOnChannel reports the misbehavior of a node on sending a message to the current node that appears
	// valid based on the networking layer but is considered invalid by the current node based on the Flow protocol.
	// The misbehavior report is sent to the current node's networking layer on the given channel to be processed.
	// Args:
	// - channel: The channel on which the misbehavior report is sent.
	// - report: The misbehavior report to be sent.
	// Returns:
	// none
	ReportMisbehaviorOnChannel(channel channels.Channel, report MisbehaviorReport)
}
