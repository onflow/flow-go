package p2p

import (
	"context"

	kbucket "github.com/libp2p/go-libp2p-kbucket"
	"github.com/libp2p/go-libp2p/core/host"
	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
)

// LibP2PNode represents a flow libp2p node. It provides the network layer with the necessary interface to
// control the underlying libp2p node. It is essentially the flow wrapper around the libp2p node, and allows
// us to define different types of libp2p nodes that can operate in different ways by overriding these methods.
// TODO: this interface is highly coupled with the current implementation of the libp2p node. We should
//
//	consider refactoring it to be more generic and less coupled with the current implementation.
//	https://github.com/dapperlabs/flow-go/issues/6575
type LibP2PNode interface {
	module.ReadyDoneAware
	Subscriptions
	// PeerConnections connection status information per peer.
	PeerConnections
	// PeerScore exposes the peer score API.
	PeerScore
	// DisallowListNotificationConsumer exposes the disallow list notification consumer API for the node so that
	// it will be notified when a new disallow list update is distributed.
	DisallowListNotificationConsumer
	// DisallowListOracle exposes the disallow list oracle API for external consumers to query about the disallow list.
	DisallowListOracle
	// Start the libp2p node.
	Start(ctx irrecoverable.SignalerContext)
	// Stop terminates the libp2p node.
	Stop() error
	// AddPeer adds a peer to this node by adding it to this node's peerstore and connecting to it.
	AddPeer(ctx context.Context, peerInfo peer.AddrInfo) error
	// RemovePeer closes the connection with the peer.
	RemovePeer(peerID peer.ID) error
	// GetPeersForProtocol returns slice peer IDs for the specified protocol ID.
	GetPeersForProtocol(pid protocol.ID) peer.IDSlice
	// CreateStream returns an existing stream connected to the peer if it exists, or creates a new stream with it.
	CreateStream(ctx context.Context, peerID peer.ID) (libp2pnet.Stream, error)
	// GetIPPort returns the IP and Port the libp2p node is listening on.
	GetIPPort() (string, string, error)
	// RoutingTable returns the node routing table
	RoutingTable() *kbucket.RoutingTable
	// ListPeers returns list of peer IDs for peers subscribed to the topic.
	ListPeers(topic string) []peer.ID
	// Subscribe subscribes the node to the given topic and returns the subscription
	Subscribe(topic channels.Topic, topicValidator TopicValidatorFunc) (Subscription, error)
	// UnSubscribe cancels the subscriber and closes the topic.
	UnSubscribe(topic channels.Topic) error
	// Publish publishes the given payload on the topic.
	Publish(ctx context.Context, topic channels.Topic, data []byte) error
	// Host returns pointer to host object of node.
	Host() host.Host
	// WithDefaultUnicastProtocol overrides the default handler of the unicast manager and registers all preferred protocols.
	WithDefaultUnicastProtocol(defaultHandler libp2pnet.StreamHandler, preferred []protocols.ProtocolName) error
	// WithPeersProvider sets the PeersProvider for the peer manager.
	// If a peer manager factory is set, this method will set the peer manager's PeersProvider.
	WithPeersProvider(peersProvider PeersProvider)
	// PeerManagerComponent returns the component interface of the peer manager.
	PeerManagerComponent() component.Component
	// RequestPeerUpdate requests an update to the peer connections of this node using the peer manager.
	RequestPeerUpdate()
	// SetRouting sets the node's routing implementation.
	// SetRouting may be called at most once.
	SetRouting(r routing.Routing)
	// Routing returns node routing object.
	Routing() routing.Routing
	// SetPubSub sets the node's pubsub implementation.
	// SetPubSub may be called at most once.
	SetPubSub(ps PubSubAdapter)
	// SetComponentManager sets the component manager for the node.
	// SetComponentManager may be called at most once.
	SetComponentManager(cm *component.ComponentManager)
}

// Subscriptions set of funcs related to current subscription info of a node.
type Subscriptions interface {
	// HasSubscription returns true if the node currently has an active subscription to the topic.
	HasSubscription(topic channels.Topic) bool
	// SetUnicastManager sets the unicast manager for the node.
	SetUnicastManager(uniMgr UnicastManager)
}

// PeerScore is the interface for the peer score module. It is used to expose the peer score to other
// components of the node. It is also used to set the peer score exposer implementation.
type PeerScore interface {
	// SetPeerScoreExposer sets the node's peer score exposer implementation.
	// SetPeerScoreExposer may be called at most once. It is an irrecoverable error to call this
	// method if the node's peer score exposer has already been set.
	SetPeerScoreExposer(e PeerScoreExposer)
	// PeerScoreExposer returns the node's peer score exposer implementation.
	// If the node's peer score exposer has not been set, the second return value will be false.
	PeerScoreExposer() (PeerScoreExposer, bool)
}

// PeerConnections subset of funcs related to underlying libp2p host connections.
type PeerConnections interface {
	// IsConnected returns true if address is a direct peer of this node else false.
	// Peers are considered not connected if the underlying libp2p host reports the
	// peers as not connected and there are no connections in the connection list.
	// The following error returns indicate a bug in the code:
	//  * network.ErrIllegalConnectionState if the underlying libp2p host reports connectedness as NotConnected but the connections list
	// 	  to the peer is not empty. This indicates a bug within libp2p.
	IsConnected(peerID peer.ID) (bool, error)
}

// DisallowListNotificationConsumer is an interface for consuming disallow/allow list update notifications.
type DisallowListNotificationConsumer interface {
	// OnDisallowListNotification is called when a new disallow list update notification is distributed.
	// Any error on consuming event must handle internally.
	// The implementation must be concurrency safe.
	// Args:
	// 	id: peer ID of the peer being disallow-listed.
	// 	cause: cause of the peer being disallow-listed (only this cause is added to the peer's disallow-listed causes).
	// Returns:
	// 	none
	OnDisallowListNotification(id peer.ID, cause network.DisallowListedCause)

	// OnAllowListNotification is called when a new allow list update notification is distributed.
	// Any error on consuming event must handle internally.
	// The implementation must be concurrency safe.
	// Args:
	// 	id: peer ID of the peer being allow-listed.
	// 	cause: cause of the peer being allow-listed (only this cause is removed from the peer's disallow-listed causes).
	// Returns:
	// 	none
	OnAllowListNotification(id peer.ID, cause network.DisallowListedCause)
}

// DisallowListOracle is an interface for querying disallow-listed peers.
type DisallowListOracle interface {
	// IsDisallowListed determines whether the given peer is disallow-listed for any reason.
	// Args:
	// - peerID: the peer to check.
	// Returns:
	// - []network.DisallowListedCause: the list of causes for which the given peer is disallow-listed. If the peer is not disallow-listed for any reason,
	// a nil slice is returned.
	// - bool: true if the peer is disallow-listed for any reason, false otherwise.
	IsDisallowListed(peerId peer.ID) ([]network.DisallowListedCause, bool)
}
