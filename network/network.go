package network

import (
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
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

// EngineRegistry is one of the networking layer interfaces in Flow (i.e., EngineRegistry, ConduitAdapter, and Underlay). It represents the interface that networking layer
// offers to the Flow protocol layer, i.e., engines. It is responsible for creating conduits through which engines
// can send and receive messages to and from other engines on the network, as well as registering other services
// such as BlobService and PingService.
type EngineRegistry interface {
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

type NoopEngineRegister struct {
	module.NoopComponent
}

func (n NoopEngineRegister) Register(channel channels.Channel, messageProcessor MessageProcessor) (Conduit, error) {
	return nil, nil
}

func (n NoopEngineRegister) RegisterBlobService(channel channels.Channel, store datastore.Batching, opts ...BlobServiceOption) (BlobService, error) {
	return nil, nil
}

func (n NoopEngineRegister) RegisterPingService(pingProtocolID protocol.ID, pingInfoProvider PingInfoProvider) (PingService, error) {
	return nil, nil
}

var _ EngineRegistry = (*NoopEngineRegister)(nil)

// ConduitAdapter is one of the networking layer interfaces in Flow (i.e., EngineRegistry, ConduitAdapter, and Underlay). It represents the interface that networking layer
// offers to a single conduit which enables the conduit to send different types of messages i.e., unicast, multicast,
// and publish, to other conduits on the network.
type ConduitAdapter interface {
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

// Underlay is one of the networking layer interfaces in Flow (i.e., EngineRegistry, ConduitAdapter, and Underlay). It represents the interface that networking layer
// offers to lower level networking components such as libp2p. It is responsible for subscribing to and unsubscribing
// from channels, as well as updating the addresses of all the authorized participants in the Flow protocol.
type Underlay interface {
	module.ReadyDoneAware
	DisallowListNotificationConsumer

	// Subscribe subscribes the network Underlay to a channel.
	// No errors are expected during normal operation.
	Subscribe(channel channels.Channel) error

	// Unsubscribe unsubscribes the network Underlay from a channel.
	// All errors returned from this function can be considered benign.
	Unsubscribe(channel channels.Channel) error

	// UpdateNodeAddresses fetches and updates the addresses of all the authorized participants
	// in the Flow protocol.
	UpdateNodeAddresses()
}

// Connection represents an interface to read from & write to a connection.
type Connection interface {
	Send(msg interface{}) error
	Receive() (interface{}, error)
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
