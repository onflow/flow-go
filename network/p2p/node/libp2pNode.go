// Package p2p encapsulates the libp2p library
package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/internal/p2putils"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/slashing"

	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/network/validator"
	flowpubsub "github.com/onflow/flow-go/network/validator/pubsub"
)

const (
	_  = iota
	kb = 1 << (10 * iota)
	mb
	gb
)

const (
	// MaxConnectAttempt is the maximum number of attempts to be made to connect to a remote node for 1-1 direct communication
	MaxConnectAttempt = 3

	// DefaultMaxPubSubMsgSize defines the maximum message size in publish and multicast modes
	DefaultMaxPubSubMsgSize = 5 * mb // 5 mb

	// timeout for FindPeer queries to the routing system
	// TODO: is this a sensible value?
	findPeerQueryTimeout = 10 * time.Second
)

// Node is a wrapper around the LibP2P host.
type Node struct {
	sync.Mutex
	uniMgr  *unicast.Manager
	host    host.Host                               // reference to the libp2p host (https://godoc.org/github.com/libp2p/go-libp2p/core/host)
	pubSub  *pubsub.PubSub                          // reference to the libp2p PubSub component
	logger  zerolog.Logger                          // used to provide logging
	topics  map[channels.Topic]*pubsub.Topic        // map of a topic string to an actual topic instance
	subs    map[channels.Topic]*pubsub.Subscription // map of a topic string to an actual subscription
	routing routing.Routing
	pCache  *ProtocolPeerCache
}

// NewNode creates a new libp2p node and sets its parameters.
func NewNode(
	logger zerolog.Logger,
	host host.Host,
	pubSub *pubsub.PubSub,
	routing routing.Routing,
	pCache *ProtocolPeerCache,
	uniMgr *unicast.Manager) *Node {

	return &Node{
		uniMgr:  uniMgr,
		host:    host,
		pubSub:  pubSub,
		logger:  logger.With().Str("component", "libp2p-node").Logger(),
		topics:  make(map[channels.Topic]*pubsub.Topic),
		subs:    make(map[channels.Topic]*pubsub.Subscription),
		routing: routing,
		pCache:  pCache,
	}
}

// Stop terminates the libp2p node.
func (n *Node) Stop() (chan struct{}, error) {
	var result error
	done := make(chan struct{})
	n.logger.Debug().Msg("unsubscribing from all topics")
	for t := range n.topics {
		if err := n.UnSubscribe(t); err != nil {
			result = multierror.Append(result, err)
		}
	}

	n.logger.Debug().Msg("stopping libp2p node")
	if err := n.host.Close(); err != nil {
		result = multierror.Append(result, err)
	}

	n.logger.Debug().Msg("closing peer store")
	// to prevent peerstore routine leak (https://github.com/libp2p/go-libp2p/issues/718)
	if err := n.host.Peerstore().Close(); err != nil {
		n.logger.Debug().Err(err).Msg("closing peer store")
		result = multierror.Append(result, err)
	}

	if result != nil {
		close(done)
		return done, result
	}

	go func(done chan struct{}) {
		defer close(done)
		addrs := len(n.host.Network().ListenAddresses())
		ticker := time.NewTicker(time.Millisecond * 2)
		defer ticker.Stop()
		timeout := time.After(time.Second)
		for addrs > 0 {
			// wait for all listen addresses to have been removed
			select {
			case <-timeout:
				n.logger.Error().Int("port", addrs).Msg("listen addresses still open")
				return
			case <-ticker.C:
				addrs = len(n.host.Network().ListenAddresses())
			}
		}

		n.logger.Debug().Msg("libp2p node stopped successfully")
	}(done)

	return done, nil
}

// AddPeer adds a peer to this node by adding it to this node's peerstore and connecting to it
func (n *Node) AddPeer(ctx context.Context, peerInfo peer.AddrInfo) error {
	return n.host.Connect(ctx, peerInfo)
}

// RemovePeer closes the connection with the peer.
func (n *Node) RemovePeer(peerID peer.ID) error {
	err := n.host.Network().ClosePeer(peerID)
	if err != nil {
		return fmt.Errorf("failed to remove peer %s: %w", peerID, err)
	}
	return nil
}

func (n *Node) GetPeersForProtocol(pid protocol.ID) peer.IDSlice {
	pMap := n.pCache.GetPeers(pid)
	peers := make(peer.IDSlice, 0, len(pMap))
	for p := range pMap {
		peers = append(peers, p)
	}
	return peers
}

// CreateStream returns an existing stream connected to the peer if it exists, or creates a new stream with it.
func (n *Node) CreateStream(ctx context.Context, peerID peer.ID) (libp2pnet.Stream, error) {
	lg := n.logger.With().Str("peer_id", peerID.Pretty()).Logger()

	// If we do not currently have any addresses for the given peer, stream creation will almost
	// certainly fail. If this Node was configured with a routing system, we can try to use it to
	// look up the address of the peer.
	if len(n.host.Peerstore().Addrs(peerID)) == 0 && n.routing != nil {
		lg.Info().Msg("address not found in peer store, searching for peer in routing system")

		var err error
		func() {
			timedCtx, cancel := context.WithTimeout(ctx, findPeerQueryTimeout)
			defer cancel()
			// try to find the peer using the routing system
			_, err = n.routing.FindPeer(timedCtx, peerID)
		}()

		if err != nil {
			lg.Warn().Err(err).Msg("address not found in both peer store and routing system")
		} else {
			lg.Debug().Msg("address not found in peer store, but found in routing system search")
		}
	}
	stream, dialAddrs, err := n.uniMgr.CreateStream(ctx, peerID, MaxConnectAttempt)
	if err != nil {
		return nil, flownet.NewPeerUnreachableError(fmt.Errorf("could not create stream (peer_id: %s, dialing address(s): %v): %w", peerID,
			dialAddrs, err))
	}

	lg.Info().
		Str("networking_protocol_id", string(stream.Protocol())).
		Str("dial_address", fmt.Sprintf("%v", dialAddrs)).
		Msg("stream successfully created to remote peer")
	return stream, nil
}

// GetIPPort returns the IP and Port the libp2p node is listening on.
func (n *Node) GetIPPort() (string, string, error) {
	return p2putils.IPPortFromMultiAddress(n.host.Network().ListenAddresses()...)
}

func (n *Node) RoutingTable() *kbucket.RoutingTable {
	return n.routing.(*dht.IpfsDHT).RoutingTable()
}

func (n *Node) ListPeers(topic string) []peer.ID {
	return n.pubSub.ListPeers(topic)
}

// Subscribe subscribes the node to the given topic and returns the subscription
// Currently only one subscriber is allowed per topic.
// NOTE: A node will receive its own published messages.
func (n *Node) Subscribe(topic channels.Topic, codec flownet.Codec, peerFilter p2p.PeerFilter, slashingViolationsConsumer slashing.ViolationsConsumer, validators ...validator.PubSubMessageValidator) (*pubsub.Subscription, error) {
	n.Lock()
	defer n.Unlock()

	// Check if the topic has been already created and is in the cache
	n.pubSub.GetTopics()
	tp, found := n.topics[topic]
	var err error
	if !found {
		topicValidator := flowpubsub.TopicValidator(n.logger, codec, slashingViolationsConsumer, peerFilter, validators...)
		if err := n.pubSub.RegisterTopicValidator(
			topic.String(), topicValidator, pubsub.WithValidatorInline(true),
		); err != nil {
			n.logger.Err(err).Str("topic", topic.String()).Msg("failed to register topic validator, aborting subscription")
			return nil, fmt.Errorf("failed to register topic validator: %w", err)
		}

		tp, err = n.pubSub.Join(topic.String())
		if err != nil {
			if err := n.pubSub.UnregisterTopicValidator(topic.String()); err != nil {
				n.logger.Err(err).Str("topic", topic.String()).Msg("failed to unregister topic validator")
			}

			return nil, fmt.Errorf("could not join topic (%s): %w", topic, err)
		}

		n.topics[topic] = tp
	}

	// Create a new subscription
	s, err := tp.Subscribe()
	if err != nil {
		return s, fmt.Errorf("could not subscribe to topic (%s): %w", topic, err)
	}

	// Add the subscription to the cache
	n.subs[topic] = s

	n.logger.Debug().
		Str("topic", topic.String()).
		Msg("subscribed to topic")
	return s, err
}

// UnSubscribe cancels the subscriber and closes the topic.
func (n *Node) UnSubscribe(topic channels.Topic) error {
	n.Lock()
	defer n.Unlock()
	// Remove the Subscriber from the cache
	if s, found := n.subs[topic]; found {
		s.Cancel()
		n.subs[topic] = nil
		delete(n.subs, topic)
	}

	tp, found := n.topics[topic]
	if !found {
		err := fmt.Errorf("could not find topic (%s)", topic)
		return err
	}

	if err := n.pubSub.UnregisterTopicValidator(topic.String()); err != nil {
		n.logger.Err(err).Str("topic", topic.String()).Msg("failed to unregister topic validator")
	}

	// attempt to close the topic
	err := tp.Close()
	if err != nil {
		err = fmt.Errorf("could not close topic (%s): %w", topic, err)
		return err
	}
	n.topics[topic] = nil
	delete(n.topics, topic)

	n.logger.Debug().
		Str("topic", topic.String()).
		Msg("unsubscribed from topic")
	return err
}

// Publish publishes the given payload on the topic
func (n *Node) Publish(ctx context.Context, topic channels.Topic, data []byte) error {
	ps, found := n.topics[topic]
	if !found {
		return fmt.Errorf("could not find topic (%s)", topic)
	}
	err := ps.Publish(ctx, data)
	if err != nil {
		return fmt.Errorf("could not publish to topic (%s): %w", topic, err)
	}
	return nil
}

// Host returns pointer to host object of node.
func (n *Node) Host() host.Host {
	return n.host
}

func (n *Node) WithDefaultUnicastProtocol(defaultHandler libp2pnet.StreamHandler, preferred []unicast.ProtocolName) error {
	n.uniMgr.WithDefaultHandler(defaultHandler)
	for _, p := range preferred {
		err := n.uniMgr.Register(p)
		if err != nil {
			return fmt.Errorf("could not register unicast protocls: %w", err)
		}
	}

	return nil
}

// IsConnected returns true is address is a direct peer of this node else false
func (n *Node) IsConnected(peerID peer.ID) (bool, error) {
	isConnected := n.host.Network().Connectedness(peerID) == libp2pnet.Connected
	return isConnected, nil
}

func (n *Node) Routing() routing.Routing {
	return n.routing
}
