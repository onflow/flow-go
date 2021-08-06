// Package libp2p encapsulates the libp2p library
package p2p

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	swarm "github.com/libp2p/go-libp2p-swarm"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/utils/logging"
)

const (

	// Maximum time to wait for a ping reply from a remote node
	PingTimeoutSecs = time.Second * 4

	// maximum number of attempts to be made to connect to a remote node for 1-1 direct communication
	maxConnectAttempt = 3
)

// LibP2PFactoryFunc is a factory function type for generating libp2p Node instances.
type LibP2PFactoryFunc func() (*Node, error)

// DefaultLibP2PNodeFactory returns a LibP2PFactoryFunc which generates the libp2p host initialized with the
// default options for the host, the pubsub and the ping service.
func DefaultLibP2PNodeFactory(ctx context.Context, log zerolog.Logger, me flow.Identifier, address string, flowKey fcrypto.PrivateKey, rootBlockID string,
	maxPubSubMsgSize int, metrics module.NetworkMetrics, pingInfoProvider PingInfoProvider) (LibP2PFactoryFunc, error) {

	connManager := NewConnManager(log, metrics)

	connGater := NewConnGater(log)

	// create PubSub options for libp2p to use
	psOptions := []pubsub.Option{
		// skip message signing
		pubsub.WithMessageSigning(false),
		// skip message signature
		pubsub.WithStrictSignatureVerification(false),
		// set max message size limit for 1-k PubSub messaging
		pubsub.WithMaxMessageSize(maxPubSubMsgSize),
		// no discovery
	}

	return func() (*Node, error) {
		libp2pHost, err := LibP2PHost(ctx, address, flowKey,
			WithLibP2PConnectionManager(connManager),
			WithLibP2PConnectionGator(connGater),
			WithLibP2PPing(true))
		if err != nil {
			return nil, err
		}

		libp2pPubSub, err := DefaultPubSub(ctx, libp2pHost, psOptions...)
		if err != nil {
			return nil, err
		}

		return NewLibP2PNode(me, rootBlockID, log, libp2pHost, libp2pPubSub,
			WithConnectionGator(connGater),
			WithConnectionManager(connManager),
			WithPingService(rootBlockID, pingInfoProvider))
	}, nil
}

// Node is a wrapper around the LibP2P host.
type Node struct {
	sync.Mutex
	connGater            *connGater                             // used to provide white listing
	host                 host.Host                              // reference to the libp2p host (https://godoc.org/github.com/libp2p/go-libp2p-core/host)
	pubSub               *pubsub.PubSub                         // reference to the libp2p PubSub component
	logger               zerolog.Logger                         // used to provide logging
	topics               map[flownet.Topic]*pubsub.Topic        // map of a topic string to an actual topic instance
	subs                 map[flownet.Topic]*pubsub.Subscription // map of a topic string to an actual subscription
	id                   flow.Identifier                        // used to represent id of flow node running this instance of libP2P node
	flowLibP2PProtocolID protocol.ID                            // the unique protocol ID
	pingService          *PingService
	connMgr              *ConnManager
}

type NodeOption func(node *Node)

// WithConnectionGator returns a NodeOption that sets the connection gater for the node
func WithConnectionGator(gater *connGater) NodeOption {
	return func(node *Node) {
		node.connGater = gater
	}
}

// WithConnectionManager returns a NodeOption that sets the connection manager for the node
func WithConnectionManager(connMgr *ConnManager) NodeOption {
	return func(node *Node) {
		node.connMgr = connMgr
	}
}

// WithPingService returns a NodeOption that initializes the PingService object for the node
func WithPingService(rootBlockID string, pingInfoProvider PingInfoProvider) NodeOption {
	return func(node *Node) {
		pingLibP2PProtocolID := generatePingProtcolID(rootBlockID)
		pingService := NewPingService(node.host, pingLibP2PProtocolID, pingInfoProvider, node.logger)
		node.pingService = pingService
	}
}

// NewLibP2PNode creates a LibP2PNode using the given id, root block ID, libp2p host and pubsub instance.
// The node can be additionally customized with the following NodeOptions:
// WithConnectionGator adds connection gating ability to the node
// WithConnectionManager adds a connection manager to receive callbacks on connection creation an destruction
// WithPingService adds a PingService to respond to a Flow Ping.
func NewLibP2PNode(id flow.Identifier,
	rootBlockID string,
	logger zerolog.Logger,
	libp2pHost host.Host,
	libp2pPubSub *pubsub.PubSub,
	options ...NodeOption) (*Node, error) {

	node := &Node{
		id:                   id,
		flowLibP2PProtocolID: generateFlowProtocolID(rootBlockID),
		host:                 libp2pHost,
		pubSub:               libp2pPubSub,
		logger:               logger,
		topics:               make(map[flownet.Topic]*pubsub.Topic),
		subs:                 make(map[flownet.Topic]*pubsub.Subscription),
	}

	for _, opt := range options {
		opt(node)
	}

	ip, port, err := node.GetIPPort()
	if err != nil {
		return nil, fmt.Errorf("failed to find IP and port on which the node was started: %w", err)
	}

	logger.Debug().
		Hex("node_id", logging.ID(id)).
		Str("address", fmt.Sprintf("%s:%s", ip, port)).
		Msg("libp2p node started successfully")

	return node, nil
}

// Stop stops the libp2p node.
func (n *Node) Stop() (chan struct{}, error) {
	var result error
	done := make(chan struct{})
	n.logger.Debug().
		Hex("node_id", logging.ID(n.id)).
		Msg("unsubscribing from all topics")
	for t := range n.topics {
		if err := n.UnSubscribe(t); err != nil {
			result = multierror.Append(result, err)
		}
	}

	n.logger.Debug().
		Hex("node_id", logging.ID(n.id)).
		Msg("stopping libp2p node")
	if err := n.host.Close(); err != nil {
		result = multierror.Append(result, err)
	}

	n.logger.Debug().
		Hex("node_id", logging.ID(n.id)).
		Msg("closing peer store")
	// to prevent peerstore routine leak (https://github.com/libp2p/go-libp2p/issues/718)
	if err := n.host.Peerstore().Close(); err != nil {
		n.logger.Debug().
			Hex("node_id", logging.ID(n.id)).
			Err(err).Msg("closing peer store")
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
		n.logger.Debug().
			Hex("node_id", logging.ID(n.id)).
			Msg("libp2p node stopped successfully")
	}(done)

	return done, nil
}

// AddPeer adds a peer to this node by adding it to this node's peerstore and connecting to it
func (n *Node) AddPeer(ctx context.Context, identity flow.Identity) error {
	pInfo, err := PeerAddressInfo(identity)
	if err != nil {
		return fmt.Errorf("failed to add peer %s: %w", identity.String(), err)
	}

	err = n.host.Connect(ctx, pInfo)
	if err != nil {
		return err
	}

	return nil
}

// RemovePeer closes the connection with the identity.
func (n *Node) RemovePeer(ctx context.Context, identity flow.Identity) error {
	pInfo, err := PeerAddressInfo(identity)
	if err != nil {
		return fmt.Errorf("failed to remove peer %x: %w", identity, err)
	}

	err = n.host.Network().ClosePeer(pInfo.ID)
	if err != nil {
		return fmt.Errorf("failed to remove peer %s: %w", identity, err)
	}
	return nil
}

// CreateStream returns an existing stream connected to identity, if it exists or adds one to identity as a peer and creates a new stream with it.
func (n *Node) CreateStream(ctx context.Context, identity flow.Identity) (libp2pnet.Stream, error) {
	// Open libp2p Stream with the remote peer (will use an existing TCP connection underneath if it exists)
	stream, err := n.tryCreateNewStream(ctx, identity, maxConnectAttempt)
	if err != nil {
		return nil, flownet.NewPeerUnreachableError(fmt.Errorf("could not create stream (node_id: %s, address: %s): %w", identity.NodeID.String(),
			identity.Address, err))
	}
	return stream, nil
}

// tryCreateNewStream makes at most maxAttempts to create a stream with the identity.
// This was put in as a fix for #2416. PubSub and 1-1 communication compete with each other when trying to connect to
// remote nodes and once in a while NewStream returns an error 'both yamux endpoints are clients'
func (n *Node) tryCreateNewStream(ctx context.Context, identity flow.Identity, maxAttempts int) (libp2pnet.Stream, error) {
	_, _, key, err := networkingInfo(identity)
	if err != nil {
		return nil, fmt.Errorf("could not get translate identity to networking info %s: %w", identity.NodeID.String(), err)
	}

	peerID, err := peer.IDFromPublicKey(key)
	if err != nil {
		return nil, fmt.Errorf("could not get peer ID: %w", err)
	}

	// protect the underlying connection from being inadvertently pruned by the peer manager while the stream and
	// connection creation is being attempted
	n.connMgr.ProtectPeer(peerID)
	// unprotect it once done
	defer n.connMgr.UnprotectPeer(peerID)

	var errs error
	var s libp2pnet.Stream
	var retries = 0
	for ; retries < maxAttempts; retries++ {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context done before stream could be created (retry attempt: %d", retries)
		default:
		}

		// remove the peer from the peer store if present
		n.host.Peerstore().ClearAddrs(peerID)

		// cancel the dial back off (if any), since we want to connect immediately
		network := n.host.Network()
		if swm, ok := network.(*swarm.Swarm); ok {
			swm.Backoff().Clear(peerID)
		}

		// if this is a retry attempt, wait for some time before retrying
		if retries > 0 {
			// choose a random interval between 0 to 5
			r := rand.Intn(5)
			time.Sleep(time.Duration(r) * time.Millisecond)
		}

		err = n.AddPeer(ctx, identity)
		if err != nil {

			// if the connection was rejected due to invalid node id, skip the re-attempt
			if strings.Contains(err.Error(), "failed to negotiate security protocol") {
				return s, fmt.Errorf("invalid node id: %w", err)
			}

			// if the connection was rejected due to allowlisting, skip the re-attempt
			if errors.Is(err, swarm.ErrGaterDisallowedConnection) {
				return s, fmt.Errorf("target node is not on the approved list of nodes: %w", err)
			}

			errs = multierror.Append(errs, err)
			continue
		}

		s, err = n.host.NewStream(ctx, peerID, n.flowLibP2PProtocolID)
		if err != nil {
			// if the stream creation failed due to invalid protocol id, skip the re-attempt
			if strings.Contains(err.Error(), "protocol not supported") {
				return nil, fmt.Errorf("remote node is running on a different spork: %w, protocol attempted: %s", err, n.flowLibP2PProtocolID)
			}
			errs = multierror.Append(errs, err)
			continue
		}

		break
	}
	if retries == maxAttempts {
		return s, errs
	}
	return s, nil
}

// GetIPPort returns the IP and Port the libp2p node is listening on.
func (n *Node) GetIPPort() (string, string, error) {
	return IPPortFromMultiAddress(n.host.Network().ListenAddresses()...)
}

// Subscribe subscribes the node to the given topic and returns the subscription
// Currently only one subscriber is allowed per topic.
// NOTE: A node will receive its own published messages.
func (n *Node) Subscribe(ctx context.Context, topic flownet.Topic) (*pubsub.Subscription, error) {
	n.Lock()
	defer n.Unlock()

	// Check if the topic has been already created and is in the cache
	n.pubSub.GetTopics()
	tp, found := n.topics[topic]
	var err error
	if !found {
		tp, err = n.pubSub.Join(topic.String())
		if err != nil {
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
		Hex("node_id", logging.ID(n.id)).
		Str("topic", topic.String()).
		Msg("subscribed to topic")
	return s, err
}

// UnSubscribe cancels the subscriber and closes the topic.
func (n *Node) UnSubscribe(topic flownet.Topic) error {
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

	// attempt to close the topic
	err := tp.Close()
	if err != nil {
		err = fmt.Errorf("could not close topic (%s): %w", topic, err)
		return err
	}
	n.topics[topic] = nil
	delete(n.topics, topic)

	n.logger.Debug().
		Hex("node_id", logging.ID(n.id)).
		Str("topic", topic.String()).
		Msg("unsubscribed from topic")
	return err
}

// Publish publishes the given payload on the topic
func (n *Node) Publish(ctx context.Context, topic flownet.Topic, data []byte) error {
	ps, found := n.topics[topic]
	if !found {
		return fmt.Errorf("could not find topic (%s)", topic)
	}
	err := ps.Publish(ctx, data)
	if err != nil {
		return fmt.Errorf("could not publish top topic (%s): %w", topic, err)
	}
	return nil
}

// Ping pings a remote node and returns the time it took to ping the remote node if successful or the error
func (n *Node) Ping(ctx context.Context, identity flow.Identity) (message.PingResponse, time.Duration, error) {

	pingError := func(err error) error {
		return fmt.Errorf("failed to ping %s (%s): %w", identity.NodeID.String(), identity.Address, err)
	}

	// convert the target node address to libp2p peer info
	targetInfo, err := PeerAddressInfo(identity)
	if err != nil {
		return message.PingResponse{}, -1, pingError(err)
	}

	n.connMgr.ProtectPeer(targetInfo.ID)
	defer n.connMgr.UnprotectPeer(targetInfo.ID)

	// connect to the target node
	err = n.host.Connect(ctx, targetInfo)
	if err != nil {
		return message.PingResponse{}, -1, pingError(err)
	}

	// ping the target
	resp, rtt, err := n.pingService.Ping(ctx, targetInfo.ID)
	if err != nil {
		return message.PingResponse{}, -1, pingError(err)
	}

	return resp, rtt, nil
}

// UpdateAllowList allows the peer allow list to be updated.
func (n *Node) UpdateAllowList(identities flow.IdentityList) error {
	// if the node was so far not under allowList
	if n.connGater == nil {
		return fmt.Errorf("Could not add an allow list, this node was started without allow listing")

	}

	// generates peer address information for all identities
	allowlist := make([]peer.AddrInfo, len(identities))
	var err error
	for i, identity := range identities {
		allowlist[i], err = PeerAddressInfo(*identity)
		if err != nil {
			return fmt.Errorf("could not generate address info: %w", err)
		}
	}

	n.connGater.update(allowlist)
	return nil
}

// Host returns pointer to host object of node.
func (n *Node) Host() host.Host {
	return n.host
}

// SetFlowProtocolStreamHandler sets the stream handler of Flow libp2p Protocol
func (n *Node) SetFlowProtocolStreamHandler(handler libp2pnet.StreamHandler) {
	n.host.SetStreamHandler(n.flowLibP2PProtocolID, handler)
}

// SetPingStreamHandler sets the stream handler for the Flow Ping protocol.
func (n *Node) SetPingStreamHandler(handler libp2pnet.StreamHandler) {
	n.host.SetStreamHandler(n.flowLibP2PProtocolID, handler)
}

// IsConnected returns true is address is a direct peer of this node else false
func (n *Node) IsConnected(identity flow.Identity) (bool, error) {
	pInfo, err := PeerAddressInfo(identity)
	if err != nil {
		return false, err
	}
	// query libp2p for connectedness status of this peer
	isConnected := n.host.Network().Connectedness(pInfo.ID) == libp2pnet.Connected
	return isConnected, nil
}

// LibP2PHost returns a libp2p host initialized to listen on the given address and using the given private key and
// customized with options
func LibP2PHost(ctx context.Context, address string, key fcrypto.PrivateKey, options ...config.Option) (host.Host, error) {

	defaultOptions, err := DefaultLibP2POptions(address, key)
	if err != nil {
		return nil, err
	}

	allOptions := append(defaultOptions, options...)

	// create the libp2p host
	libP2PHost, err := libp2p.New(ctx, allOptions...)
	if err != nil {
		return nil, fmt.Errorf("could not create libp2p host: %w", err)
	}

	return libP2PHost, nil

}

func WithLibP2PPing(enable bool) config.Option {
	return libp2p.Ping(enable)
}

func WithLibP2PConnectionManager(connMgr *ConnManager) config.Option {
	return libp2p.ConnectionManager(connMgr)
}

func WithLibP2PConnectionGator(connGater *connGater) config.Option {
	return libp2p.ConnectionGater(connGater)
}

// DefaultLibP2POptions creates and returns the standard LibP2P host options that are used for the Flow Libp2p network
func DefaultLibP2POptions(address string, key fcrypto.PrivateKey) ([]config.Option, error) {

	libp2pKey, err := PrivKey(key)
	if err != nil {
		return nil, fmt.Errorf("could not generate libp2p key: %w", err)
	}

	ip, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("could not split node address %s:%w", address, err)
	}

	sourceMultiAddr, err := multiaddr.NewMultiaddr(MultiAddressStr(ip, port))
	if err != nil {
		return nil, fmt.Errorf("failed to translate Flow address to Libp2p multiaddress: %w", err)
	}

	// create a transport which disables port reuse and web socket.
	// Port reuse enables listening and dialing from the same TCP port (https://github.com/libp2p/go-reuseport)
	// While this sounds great, it intermittently causes a 'broken pipe' error
	// as the 1-k discovery process and the 1-1 messaging both sometimes attempt to open connection to the same target
	// As of now there is no requirement of client sockets to be a well-known port, so disabling port reuse all together.
	transport := libp2p.Transport(func(u *tptu.Upgrader) *tcp.TcpTransport {
		tpt := tcp.NewTCPTransport(u)
		tpt.DisableReuseport = true
		return tpt
	})

	// gather all the options for the libp2p node
	options := []config.Option{
		libp2p.ListenAddrs(sourceMultiAddr), // set the listen address
		libp2p.Identity(libp2pKey),          // pass in the networking key
		transport,                           // set the protocol
	}

	return options, nil
}

// DefaultPubSub returns initializes and returns a GossipSub object for the given libp2p host and options
func DefaultPubSub(ctx context.Context, host host.Host, psOption ...pubsub.Option) (*pubsub.PubSub, error) {
	// Creating a new PubSub instance of the type GossipSub with psOption
	pubSub, err := pubsub.NewGossipSub(ctx, host, psOption...)
	if err != nil {
		return nil, fmt.Errorf("could not create libp2p gossipsub: %w", err)
	}
	return pubSub, nil
}
