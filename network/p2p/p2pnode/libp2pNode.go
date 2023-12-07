// Package p2pnode encapsulates the libp2p library
package p2pnode

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-multierror"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	"github.com/libp2p/go-libp2p/core/host"
	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2putils"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
	"github.com/onflow/flow-go/network/p2p/p2pnode/internal"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	_ = iota
	_ = 1 << (10 * iota)
	mb
)

const (
	// DefaultMaxPubSubMsgSize defines the maximum message size in publish and multicast modes
	DefaultMaxPubSubMsgSize = 5 * mb // 5 mb

	// timeout for FindPeer queries to the routing system
	// TODO: is this a sensible value?
	findPeerQueryTimeout = 10 * time.Second
)

var _ p2p.LibP2PNode = (*Node)(nil)

// Node is a wrapper around the LibP2P host.
type Node struct {
	component.Component
	sync.RWMutex
	uniMgr      p2p.UnicastManager
	host        host.Host // reference to the libp2p host (https://godoc.org/github.com/libp2p/go-libp2p/core/host)
	pubSub      p2p.PubSubAdapter
	logger      zerolog.Logger                      // used to provide logging
	topics      map[channels.Topic]p2p.Topic        // map of a topic string to an actual topic instance
	subs        map[channels.Topic]p2p.Subscription // map of a topic string to an actual subscription
	routing     routing.Routing
	pCache      p2p.ProtocolPeerCache
	peerManager p2p.PeerManager
	// Cache of temporary disallow-listed peers, when a peer is disallow-listed, the connections to that peer
	// are closed and further connections are not allowed till the peer is removed from the disallow-list.
	disallowListedCache p2p.DisallowListCache
	parameters          *p2p.NodeParameters
}

// NewNode creates a new libp2p node and sets its parameters.
// Args:
//   - cfg: The configuration for the libp2p node.
//
// Returns:
//   - *Node: The created libp2p node.
//
// - error: An error, if any occurred during the process. This includes failure in creating the node. The returned error is irrecoverable, and the node cannot be used.
func NewNode(cfg *p2p.NodeConfig) (*Node, error) {
	err := validator.New().Struct(cfg)
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	pCache, err := internal.NewProtocolPeerCache(cfg.Logger, cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to create protocol peer cache: %w", err)
	}

	return &Node{
		host:        cfg.Host,
		logger:      cfg.Logger.With().Str("component", "libp2p-node").Logger(),
		topics:      make(map[channels.Topic]p2p.Topic),
		subs:        make(map[channels.Topic]p2p.Subscription),
		pCache:      pCache,
		peerManager: cfg.PeerManager,
		disallowListedCache: internal.NewDisallowListCache(
			cfg.DisallowListCacheCfg.MaxSize,
			cfg.Logger.With().Str("module", "disallow-list-cache").Logger(),
			cfg.DisallowListCacheCfg.Metrics,
		),
	}, nil
}

func (n *Node) Start(ctx irrecoverable.SignalerContext) {
	n.Component.Start(ctx)
}

// Stop terminates the libp2p node.
// All errors returned from this function can be considered benign.
func (n *Node) Stop() error {
	var result error

	n.logger.Debug().Msg("unsubscribing from all topics")
	for t := range n.topics {
		err := n.unsubscribeTopic(t)
		// context cancelled errors are expected while unsubscribing from topics during shutdown
		if err != nil && !errors.Is(err, context.Canceled) {
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
		return result
	}

	addrs := len(n.host.Network().ListenAddresses())
	ticker := time.NewTicker(time.Millisecond * 2)
	defer ticker.Stop()
	timeout := time.After(time.Second)
	for addrs > 0 {
		// wait for all listen addresses to have been removed
		select {
		case <-timeout:
			n.logger.Error().Int("port", addrs).Msg("listen addresses still open")
			return nil
		case <-ticker.C:
			addrs = len(n.host.Network().ListenAddresses())
		}
	}

	n.logger.Debug().Msg("libp2p node stopped successfully")

	return nil
}

// ConnectToPeerAddrInfo adds a peer to this node by adding it to this node's peerstore and connecting to it.
// All errors returned from this function can be considered benign.
func (n *Node) ConnectToPeer(ctx context.Context, peerInfo peer.AddrInfo) error {
	return n.host.Connect(ctx, peerInfo)
}

// RemovePeer closes the connection with the peer.
// All errors returned from this function can be considered benign.
func (n *Node) RemovePeer(peerID peer.ID) error {
	err := n.host.Network().ClosePeer(peerID)
	if err != nil {
		return fmt.Errorf("failed to remove peer %s: %w", peerID, err)
	}
	// logging with suspicious level as we only expect to disconnect from a peer if it is not part of the
	// protocol state.
	n.logger.Warn().
		Str("peer_id", p2plogging.PeerId(peerID)).
		Bool(logging.KeySuspicious, true).
		Msg("disconnected from peer")

	return nil
}

// GetPeersForProtocol returns slice peer IDs for the specified protocol ID.
func (n *Node) GetPeersForProtocol(pid protocol.ID) peer.IDSlice {
	pMap := n.pCache.GetPeers(pid)
	peers := make(peer.IDSlice, 0, len(pMap))
	for p := range pMap {
		peers = append(peers, p)
	}
	return peers
}

// OpenProtectedStream opens a new stream to a peer with a protection tag. The protection tag can be used to ensure
// that the connection to the peer is maintained for a particular purpose. The stream is opened to the given peerID
// and writingLogic is executed on the stream. The created stream does not need to be reused and can be inexpensively
// created for each send. Moreover, the stream creation does not incur a round-trip time as the stream negotiation happens
// on an existing connection.
//
// Args:
//   - ctx: The context used to control the stream's lifecycle.
//   - peerID: The ID of the peer to open the stream to.
//   - protectionTag: A tag that protects the connection and ensures that the connection manager keeps it alive, and
//     won't prune the connection while the tag is active.
//   - writingLogic: A callback function that contains the logic for writing to the stream. It allows an external caller to
//     write to the stream without having to worry about the stream creation and management.
//
// Returns:
// error: An error, if any occurred during the process. This includes failure in creating the stream, setting the write
// deadline, executing the writing logic, resetting the stream if the writing logic fails, or closing the stream.
// All returned errors during this process can be considered benign.
func (n *Node) OpenProtectedStream(ctx context.Context, peerID peer.ID, protectionTag string, writingLogic func(stream libp2pnet.Stream) error) error {
	n.host.ConnManager().Protect(peerID, protectionTag)
	defer n.host.ConnManager().Unprotect(peerID, protectionTag)

	// streams don't need to be reused and are fairly inexpensive to be created for each send.
	// A stream creation does NOT incur an RTT as stream negotiation happens on an existing connection.
	s, err := n.createStream(ctx, peerID)
	if err != nil {
		return fmt.Errorf("failed to create stream for %s: %w", peerID, err)
	}

	deadline, _ := ctx.Deadline()
	err = s.SetWriteDeadline(deadline)
	if err != nil {
		return fmt.Errorf("failed to set write deadline for stream: %w", err)
	}

	err = writingLogic(s)
	if err != nil {
		// reset the stream to ensure that the next stream creation is not affected by the error.
		resetErr := s.Reset()
		if resetErr != nil {
			n.logger.Error().
				Str("target_peer_id", p2plogging.PeerId(peerID)).
				Err(resetErr).
				Msg("failed to reset stream")
		}

		return fmt.Errorf("writing logic failed for %s: %w", peerID, err)
	}

	// close the stream immediately
	err = s.Close()
	if err != nil {
		return fmt.Errorf("failed to close the stream for %s: %w", peerID, err)
	}

	return nil
}

// createStream creates a new stream to the given peer.
// Args:
//   - ctx: The context used to control the stream's lifecycle.
//   - peerID: The ID of the peer to open the stream to.
//
// Returns:
//   - libp2pnet.Stream: The created stream.
//   - error: An error, if any occurred during the process. This includes failure in creating the stream. All returned
//     errors during this process can be considered benign.
func (n *Node) createStream(ctx context.Context, peerID peer.ID) (libp2pnet.Stream, error) {
	lg := n.logger.With().Str("peer_id", p2plogging.PeerId(peerID)).Logger()

	// If we do not currently have any addresses for the given peer, stream creation will almost
	// certainly fail. If this Node was configured with a routing system, we can try to use it to
	// look up the address of the peer.
	if len(n.host.Peerstore().Addrs(peerID)) == 0 && n.routing != nil {
		lg.Debug().Msg("address not found in peer store, searching for peer in routing system")

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

	stream, err := n.uniMgr.CreateStream(ctx, peerID)
	if err != nil {
		return nil, flownet.NewPeerUnreachableError(fmt.Errorf("could not create stream peer_id: %s: %w", peerID, err))
	}

	lg.Info().
		Str("networking_protocol_id", string(stream.Protocol())).
		Msg("stream successfully created to remote peer")
	return stream, nil
}

// ID returns the peer.ID of the node, which is the unique identifier of the node at the libp2p level.
// For other libp2p nodes, the current node is identified by this ID.
func (n *Node) ID() peer.ID {
	return n.host.ID()
}

// GetIPPort returns the IP and Port the libp2p node is listening on.
// All errors returned from this function can be considered benign.
func (n *Node) GetIPPort() (string, string, error) {
	return p2putils.IPPortFromMultiAddress(n.host.Network().ListenAddresses()...)
}

// RoutingTable returns the node routing table
func (n *Node) RoutingTable() *kbucket.RoutingTable {
	return n.routing.(*dht.IpfsDHT).RoutingTable()
}

// ListPeers returns list of peer IDs for peers subscribed to the topic.
func (n *Node) ListPeers(topic string) []peer.ID {
	return n.pubSub.ListPeers(topic)
}

// Subscribe subscribes the node to the given topic and returns the subscription
// All errors returned from this function can be considered benign.
func (n *Node) Subscribe(topic channels.Topic, topicValidator p2p.TopicValidatorFunc) (p2p.Subscription, error) {
	n.Lock()
	defer n.Unlock()

	// Check if the topic has been already created and is in the cache
	n.pubSub.GetTopics()
	tp, found := n.topics[topic]
	var err error
	if !found {
		if err := n.pubSub.RegisterTopicValidator(topic.String(), topicValidator); err != nil {
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

// Unsubscribe cancels the subscriber and closes the topic.
// Args:
// topic: topic to unsubscribe from.
// Returns:
// error: error if any, which means unsubscribe failed.
// All errors returned from this function can be considered benign.
func (n *Node) Unsubscribe(topic channels.Topic) error {
	err := n.unsubscribeTopic(topic)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe from topic: %w", err)
	}

	n.RequestPeerUpdate()

	return nil
}

// unsubscribeTopic cancels the subscriber and closes the topic.
// All errors returned from this function can be considered benign.
// Args:
//
//	topic: topic to unsubscribe from
//
// Returns:
// error: error if any.
func (n *Node) unsubscribeTopic(topic channels.Topic) error {
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
		return fmt.Errorf("failed to unregister topic validator: %w", err)
	}

	// attempt to close the topic
	err := tp.Close()
	if err != nil {
		return fmt.Errorf("could not close topic (%s): %w", topic, err)
	}
	n.topics[topic] = nil
	delete(n.topics, topic)

	n.logger.Debug().
		Str("topic", topic.String()).
		Msg("unsubscribed from topic")

	return nil
}

// Publish publishes the given payload on the topic.
// All errors returned from this function can be considered benign.
func (n *Node) Publish(ctx context.Context, messageScope flownet.OutgoingMessageScope) error {
	lg := n.logger.With().
		Str("topic", messageScope.Topic().String()).
		Interface("proto_message", messageScope.Proto()).
		Str("payload_type", messageScope.PayloadType()).
		Int("message_size", messageScope.Size()).Logger()
	lg.Debug().Msg("received message to publish")

	// convert the message to bytes to be put on the wire.
	data, err := messageScope.Proto().Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal the message: %w", err)
	}

	msgSize := len(data)
	if msgSize > DefaultMaxPubSubMsgSize {
		// libp2p pubsub will silently drop the message if its size is greater than the configured pubsub max message size
		// hence return an error as this message is undeliverable
		return fmt.Errorf("message size %d exceeds configured max message size %d", msgSize, DefaultMaxPubSubMsgSize)
	}

	ps, found := n.topics[messageScope.Topic()]
	if !found {
		return fmt.Errorf("could not find topic (%s)", messageScope.Topic())
	}
	err = ps.Publish(ctx, data)
	if err != nil {
		return fmt.Errorf("could not publish to topic (%s): %w", messageScope.Topic(), err)
	}

	lg.Debug().Msg("published message to topic")
	return nil
}

// HasSubscription returns true if the node currently has an active subscription to the topic.
func (n *Node) HasSubscription(topic channels.Topic) bool {
	n.RLock()
	defer n.RUnlock()
	_, ok := n.subs[topic]
	return ok
}

// Host returns pointer to host object of node.
func (n *Node) Host() host.Host {
	return n.host
}

// WithDefaultUnicastProtocol overrides the default handler of the unicast manager and registers all preferred protocols.
func (n *Node) WithDefaultUnicastProtocol(defaultHandler libp2pnet.StreamHandler, preferred []protocols.ProtocolName) error {
	n.uniMgr.SetDefaultHandler(defaultHandler)
	for _, p := range preferred {
		err := n.uniMgr.Register(p)
		if err != nil {
			return fmt.Errorf("could not register unicast protocls: %w", err)
		}
	}

	return nil
}

// WithPeersProvider sets the PeersProvider for the peer manager.
// If a peer manager factory is set, this method will set the peer manager's PeersProvider.
func (n *Node) WithPeersProvider(peersProvider p2p.PeersProvider) {
	// TODO: chore: we should not allow overriding the peers provider if one is already set.
	if n.peerManager != nil {
		n.peerManager.SetPeersProvider(
			func() peer.IDSlice {
				authorizedPeersIds := peersProvider()
				allowListedPeerIds := peer.IDSlice{} // subset of authorizedPeersIds that are not disallowed
				for _, peerId := range authorizedPeersIds {
					// exclude the disallowed peers from the authorized peers list
					causes, disallowListed := n.disallowListedCache.IsDisallowListed(peerId)
					if disallowListed {
						n.logger.Warn().
							Str("peer_id", p2plogging.PeerId(peerId)).
							Str("causes", fmt.Sprintf("%v", causes)).
							Msg("peer is disallowed for a cause, removing from authorized peers of peer manager")

						// exclude the peer from the authorized peers list
						continue
					}
					allowListedPeerIds = append(allowListedPeerIds, peerId)
				}

				return allowListedPeerIds
			},
		)
	}
}

// PeerManagerComponent returns the component interface of the peer manager.
func (n *Node) PeerManagerComponent() component.Component {
	return n.peerManager
}

// RequestPeerUpdate requests an update to the peer connections of this node using the peer manager.
func (n *Node) RequestPeerUpdate() {
	if n.peerManager != nil {
		n.peerManager.RequestPeerUpdate()
	}
}

// IsConnected returns true if address is a direct peer of this node else false.
// Peers are considered not connected if the underlying libp2p host reports the
// peers as not connected and there are no connections in the connection list.
// error returns:
//   - network.ErrIllegalConnectionState if the underlying libp2p host reports connectedness as NotConnected but the connections list
//     to the peer is not empty. This would normally indicate a bug within libp2p. Although the network.ErrIllegalConnectionState a bug in libp2p there is a small chance that this error will be returned due
//     to a race condition between the time we check Connectedness and ConnsToPeer. There is a chance that a connection could be established
//     after we check Connectedness but right before we check ConnsToPeer.
func (n *Node) IsConnected(peerID peer.ID) (bool, error) {
	isConnected := n.host.Network().Connectedness(peerID)
	numOfConns := len(n.host.Network().ConnsToPeer(peerID))
	if isConnected == libp2pnet.NotConnected && numOfConns > 0 {
		return true, flownet.NewConnectionStatusErr(peerID, numOfConns)
	}
	return isConnected == libp2pnet.Connected && numOfConns > 0, nil
}

// SetRouting sets the node's routing implementation.
// SetRouting may be called at most once.
func (n *Node) SetRouting(r routing.Routing) error {
	if n.routing != nil {
		// we should not allow overriding the routing implementation if one is already set; crashing the node.
		return fmt.Errorf("routing already set")
	}

	n.routing = r
	return nil
}

// Routing returns the node's routing implementation.
func (n *Node) Routing() routing.Routing {
	return n.routing
}

// PeerScoreExposer returns the node's peer score exposer implementation.
// If the node's peer score exposer has not been set, the second return value will be false.
func (n *Node) PeerScoreExposer() p2p.PeerScoreExposer {
	return n.pubSub.PeerScoreExposer()
}

// SetPubSub sets the node's pubsub implementation.
// SetPubSub may be called at most once.
func (n *Node) SetPubSub(ps p2p.PubSubAdapter) {
	if n.pubSub != nil {
		n.logger.Fatal().Msg("pubSub already set")
	}

	n.pubSub = ps
}

// GetLocalMeshPeers returns the list of peers in the local mesh for the given topic.
// Args:
// - topic: the topic.
// Returns:
// - []peer.ID: the list of peers in the local mesh for the given topic.
func (n *Node) GetLocalMeshPeers(topic channels.Topic) []peer.ID {
	return n.pubSub.GetLocalMeshPeers(topic)
}

// SetComponentManager sets the component manager for the node.
// SetComponentManager may be called at most once.
func (n *Node) SetComponentManager(cm *component.ComponentManager) {
	if n.Component != nil {
		n.logger.Fatal().Msg("component already set")
	}

	n.Component = cm
}

// SetUnicastManager sets the unicast manager for the node.
// SetUnicastManager may be called at most once.
func (n *Node) SetUnicastManager(uniMgr p2p.UnicastManager) {
	if n.uniMgr != nil {
		n.logger.Fatal().Msg("unicast manager already set")
	}
	n.uniMgr = uniMgr
}

// OnDisallowListNotification is called when a new disallow list update notification is distributed.
// Any error on consuming event must handle internally.
// The implementation must be concurrency safe.
// Args:
//
//	id: peer ID of the peer being disallow-listed.
//	cause: cause of the peer being disallow-listed (only this cause is added to the peer's disallow-listed causes).
//
// Returns:
//
//	none
func (n *Node) OnDisallowListNotification(peerId peer.ID, cause flownet.DisallowListedCause) {
	causes, err := n.disallowListedCache.DisallowFor(peerId, cause)
	if err != nil {
		// returned error is fatal.
		n.logger.Fatal().Err(err).Str("peer_id", p2plogging.PeerId(peerId)).Msg("failed to add peer to disallow list")
	}

	// TODO: this code should further be refactored to also log the Flow id.
	n.logger.Warn().
		Str("peer_id", p2plogging.PeerId(peerId)).
		Str("notification_cause", cause.String()).
		Str("causes", fmt.Sprintf("%v", causes)).
		Msg("peer added to disallow list cache")
}

// OnAllowListNotification is called when a new allow list update notification is distributed.
// Any error on consuming event must handle internally.
// The implementation must be concurrency safe.
// Args:
//
//	id: peer ID of the peer being allow-listed.
//	cause: cause of the peer being allow-listed (only this cause is removed from the peer's disallow-listed causes).
//
// Returns:
//
//	none
func (n *Node) OnAllowListNotification(peerId peer.ID, cause flownet.DisallowListedCause) {
	remainingCauses := n.disallowListedCache.AllowFor(peerId, cause)

	n.logger.Debug().
		Str("peer_id", p2plogging.PeerId(peerId)).
		Str("causes", fmt.Sprintf("%v", cause)).
		Str("remaining_causes", fmt.Sprintf("%v", remainingCauses)).
		Msg("peer is allow-listed for cause")
}

// IsDisallowListed determines whether the given peer is disallow-listed for any reason.
// Args:
// - peerID: the peer to check.
// Returns:
// - []network.DisallowListedCause: the list of causes for which the given peer is disallow-listed. If the peer is not disallow-listed for any reason,
// a nil slice is returned.
// - bool: true if the peer is disallow-listed for any reason, false otherwise.
func (n *Node) IsDisallowListed(peerId peer.ID) ([]flownet.DisallowListedCause, bool) {
	return n.disallowListedCache.IsDisallowListed(peerId)
}

// ActiveClustersChanged is called when the active clusters list of the collection clusters has changed.
// The LibP2PNode implementation directly calls the ActiveClustersChanged method of the pubsub implementation, as
// the pubsub implementation is responsible for the actual handling of the event.
// Args:
// - list: the new active clusters list.
// Returns:
// - none
func (n *Node) ActiveClustersChanged(list flow.ChainIDList) {
	n.pubSub.ActiveClustersChanged(list)
}
