package p2pnode

import (
	"context"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/unicast"
)

type LibP2PNode interface {
	// Stop terminates the libp2p node.
	Stop() (chan struct{}, error)
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
	Subscribe(topic channels.Topic, topicValidator pubsub.ValidatorEx) (*pubsub.Subscription, error)
	// UnSubscribe cancels the subscriber and closes the topic.
	UnSubscribe(topic channels.Topic) error
	// Publish publishes the given payload on the topic.
	Publish(ctx context.Context, topic channels.Topic, data []byte) error
	// Host returns pointer to host object of node.
	Host() host.Host
	// WithDefaultUnicastProtocol overrides the default handler of the unicast manager and registers all preferred protocols.
	WithDefaultUnicastProtocol(defaultHandler libp2pnet.StreamHandler, preferred []unicast.ProtocolName) error
	// IsConnected returns true is address is a direct peer of this node else false.
	IsConnected(peerID peer.ID) (bool, error)
	// Routing returns node routing object.
	Routing() routing.Routing
}
