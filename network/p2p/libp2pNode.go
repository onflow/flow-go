// Package p2p encapsulates the libp2p library
package p2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/host"
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p/dns"
	"github.com/onflow/flow-go/network/p2p/unicast"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/logging"
)

const (

	// PingTimeout is maximum time to wait for a ping reply from a remote node
	PingTimeout = time.Second * 4

	// maximum number of attempts to be made to connect to a remote node for 1-1 direct communication
	maxConnectAttempt = 3

	// timeout for FindPeer queries to the DHT
	// TODO: is this a sensible value?
	findPeerQueryTimeout = 10 * time.Second
)

// LibP2PFactoryFunc is a factory function type for generating libp2p Node instances.
type LibP2PFactoryFunc func(context.Context) (*Node, error)

// LibP2PStreamCompressorWrapperFunc is a wrapper function that plugs in compression for streams.
type LibP2PStreamCompressorWrapperFunc func(libp2pnet.Stream) (libp2pnet.Stream, error)

// WithoutCompression reflects the input stream without a compression.
func WithoutCompression(s libp2pnet.Stream) (libp2pnet.Stream, error) {
	return s, nil
}

// WithGzipCompression creates and returns a gzip-compressed stream out of input stream.
func WithGzipCompression(s libp2pnet.Stream) (libp2pnet.Stream, error) {
	return compressed.NewCompressedStream(s, compressor.GzipStreamCompressor{})
}

// LibP2PStreamCompressorFactoryFunc translates name of a stream factory to its corresponding stream compressor factory
// function.
func LibP2PStreamCompressorFactoryFunc(factory string) (LibP2PStreamCompressorWrapperFunc, error) {
	switch factory {
	case NoCompression:
		return WithoutCompression, nil
	case GzipCompression:
		return WithGzipCompression, nil
	default:
		return nil, fmt.Errorf("unknown stream factory: %s", factory)
	}
}

// DefaultLibP2PNodeFactory returns a LibP2PFactoryFunc which generates the libp2p host initialized with the
// default options for the host, the pubsub and the ping service.
func DefaultLibP2PNodeFactory(
	log zerolog.Logger,
	me flow.Identifier,
	address string,
	flowKey fcrypto.PrivateKey,
	sporkID flow.Identifier,
	idProvider id.IdentityProvider,
	maxPubSubMsgSize int,
	metrics module.NetworkMetrics,
	pingInfoProvider PingInfoProvider,
	dnsResolverTTL time.Duration,
	role string,
	streamFactory LibP2PStreamCompressorWrapperFunc,
) (LibP2PFactoryFunc, error) {

	connManager := NewConnManager(log, metrics)

	connGater := NewConnGater(log)

	resolver := dns.NewResolver(metrics, dns.WithTTL(dnsResolverTTL))

	psOpts := DefaultPubsubOptions(maxPubSubMsgSize)

	if role != "ghost" {
		psOpts = append(psOpts, func(_ context.Context, h host.Host) (pubsub.Option, error) {
			return pubsub.WithSubscriptionFilter(NewRoleBasedFilter(h.ID(), idProvider)), nil
		})
	}

	return func(ctx context.Context) (*Node, error) {
		return NewDefaultLibP2PNodeBuilder(me, address, flowKey).
			SetSporkID(sporkID).
			SetConnectionGater(connGater).
			SetConnectionManager(connManager).
			SetPubsubOptions(psOpts...).
			SetPingInfoProvider(pingInfoProvider).
			SetLogger(log).
			SetResolver(resolver).
			SetStreamCompressor(streamFactory).
			Build(ctx)
	}, nil
}

type NodeBuilder interface {
	SetSporkID(flow.Identifier) NodeBuilder
	SetConnectionManager(connmgr.ConnManager) NodeBuilder
	SetConnectionGater(*ConnGater) NodeBuilder
	SetPubsubOptions(...PubsubOption) NodeBuilder
	SetPingInfoProvider(PingInfoProvider) NodeBuilder
	SetDHTOptions(...dht.Option) NodeBuilder
	SetTopicValidation(bool) NodeBuilder
	SetLogger(zerolog.Logger) NodeBuilder
	SetResolver(*dns.Resolver) NodeBuilder
	SetStreamCompressor(LibP2PStreamCompressorWrapperFunc) NodeBuilder
	Build(context.Context) (*Node, error)
}

type DefaultLibP2PNodeBuilder struct {
	id               flow.Identifier
	sporkID          flow.Identifier
	logger           zerolog.Logger
	connGater        *ConnGater
	connMngr         connmgr.ConnManager
	pingInfoProvider PingInfoProvider
	resolver         *dns.Resolver
	streamFactory    LibP2PStreamCompressorWrapperFunc
	pubSubMaker      func(context.Context, host.Host, ...pubsub.Option) (*pubsub.PubSub, error)
	hostMaker        func(context.Context, ...config.Option) (host.Host, error)
	pubSubOpts       []PubsubOption
	dhtOpts          []dht.Option
	topicValidation  bool
}

func NewDefaultLibP2PNodeBuilder(id flow.Identifier, address string, flowKey fcrypto.PrivateKey) NodeBuilder {
	return &DefaultLibP2PNodeBuilder{
		id: id,
		pubSubMaker: func(ctx context.Context, h host.Host, opts ...pubsub.Option) (*pubsub.PubSub, error) {
			return DefaultPubSub(ctx, h, opts...)
		},
		hostMaker: func(ctx context.Context, opts ...config.Option) (host.Host, error) {
			return DefaultLibP2PHost(ctx, address, flowKey, opts...)
		},
		topicValidation: true,
	}
}

func (builder *DefaultLibP2PNodeBuilder) SetDHTOptions(opts ...dht.Option) NodeBuilder {
	builder.dhtOpts = opts
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetTopicValidation(enabled bool) NodeBuilder {
	builder.topicValidation = enabled
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetSporkID(sporkID flow.Identifier) NodeBuilder {
	builder.sporkID = sporkID
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetConnectionManager(connMngr connmgr.ConnManager) NodeBuilder {
	builder.connMngr = connMngr
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetConnectionGater(connGater *ConnGater) NodeBuilder {
	builder.connGater = connGater
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetPubsubOptions(opts ...PubsubOption) NodeBuilder {
	builder.pubSubOpts = opts
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetPingInfoProvider(pingInfoProvider PingInfoProvider) NodeBuilder {
	builder.pingInfoProvider = pingInfoProvider
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetLogger(logger zerolog.Logger) NodeBuilder {
	builder.logger = logger
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetResolver(resolver *dns.Resolver) NodeBuilder {
	builder.resolver = resolver
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetStreamCompressor(streamFactory LibP2PStreamCompressorWrapperFunc) NodeBuilder {
	builder.streamFactory = streamFactory
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) Build(ctx context.Context) (*Node, error) {
	node := &Node{
		id:               builder.id,
		topics:           make(map[flownet.Topic]*pubsub.Topic),
		subs:             make(map[flownet.Topic]*pubsub.Subscription),
		logger:           builder.logger,
		topicValidation:  builder.topicValidation,
		compressedStream: WithoutCompression,
	}

	if builder.hostMaker == nil {
		return nil, errors.New("unable to create libp2p host: factory function not provided")
	}

	if builder.pubSubMaker == nil {
		return nil, errors.New("unable to create libp2p pubsub: factory function not provided")
	}

	if builder.sporkID == flow.ZeroID {
		return nil, errors.New("spork ID must be provided")
	}
	node.flowLibP2PProtocolID = FlowProtocolID(builder.sporkID)

	var opts []config.Option

	if builder.connGater != nil {
		opts = append(opts, libp2p.ConnectionGater(builder.connGater))
		node.connGater = builder.connGater
	}

	if builder.connMngr != nil {
		opts = append(opts, libp2p.ConnectionManager(builder.connMngr))
		node.connMgr = builder.connMngr
	}

	if builder.pingInfoProvider != nil {
		opts = append(opts, libp2p.Ping(true))
	}

	if builder.resolver != nil { // sets DNS resolver
		libp2pResolver, err := madns.NewResolver(madns.WithDefaultResolver(builder.resolver))
		if err != nil {
			return nil, fmt.Errorf("could not create libp2p resolver: %w", err)
		}

		select {
		case <-builder.resolver.Ready():
		case <-time.After(30 * time.Second):
			return nil, fmt.Errorf("could not start resolver on time")
		}

		opts = append(opts, libp2p.MultiaddrResolver(libp2pResolver))
	}

	if builder.streamFactory != nil {
		node.compressedStream = builder.streamFactory
	}

	libp2pHost, err := builder.hostMaker(ctx, opts...)
	if err != nil {
		return nil, err
	}
	node.host = libp2pHost

	node.pCache, err = newProtocolPeerCache(node.logger, libp2pHost)
	if err != nil {
		return nil, err
	}

	if len(builder.dhtOpts) != 0 {
		kdht, err := NewDHT(ctx, node.host, builder.dhtOpts...)
		if err != nil {
			return nil, err
		}
		node.dht = kdht
		builder.pubSubOpts = append(builder.pubSubOpts, WithDHTDiscovery(kdht))
	}

	if builder.pingInfoProvider != nil {
		pingLibP2PProtocolID := generatePingProtocolID(builder.sporkID)
		pingService := NewPingService(libp2pHost, pingLibP2PProtocolID, builder.pingInfoProvider, node.logger)
		node.pingService = pingService
	}

	var libp2pPSOptions []pubsub.Option
	// generate the libp2p Pubsub options from the given context and host
	for _, optionGenerator := range builder.pubSubOpts {
		option, err := optionGenerator(ctx, libp2pHost)
		if err != nil {
			return nil, err
		}
		libp2pPSOptions = append(libp2pPSOptions, option)
	}

	ps, err := builder.pubSubMaker(ctx, libp2pHost, libp2pPSOptions...)
	if err != nil {
		return nil, err
	}
	node.pubSub = ps

	ip, port, err := node.GetIPPort()
	if err != nil {
		return nil, fmt.Errorf("failed to find IP and port on which the node was started: %w", err)
	}

	node.logger.Debug().
		Hex("node_id", logging.ID(node.id)).
		Str("address", fmt.Sprintf("%s:%s", ip, port)).
		Msg("libp2p node started successfully")

	return node, nil
}

// Node is a wrapper around the LibP2P host.
type Node struct {
	sync.Mutex
	unicastManager       *unicast.Manager
	connGater            *ConnGater                             // used to provide white listing
	host                 host.Host                              // reference to the libp2p host (https://godoc.org/github.com/libp2p/go-libp2p-core/host)
	pubSub               *pubsub.PubSub                         // reference to the libp2p PubSub component
	logger               zerolog.Logger                         // used to provide logging
	topics               map[flownet.Topic]*pubsub.Topic        // map of a topic string to an actual topic instance
	subs                 map[flownet.Topic]*pubsub.Subscription // map of a topic string to an actual subscription
	id                   flow.Identifier                        // used to represent id of flow node running this instance of libP2P node
	flowLibP2PProtocolID protocol.ID                            // the unique protocol ID
	resolver             *dns.Resolver                          // dns resolver for libp2p (is nil if default)
	pingService          *PingService
	connMgr              connmgr.ConnManager
	dht                  *dht.IpfsDHT
	topicValidation      bool
	pCache               *protocolPeerCache
}

// Stop terminates the libp2p node.
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

		if n.resolver != nil {
			// non-nil resolver means a non-default one, so it must be stopped.
			n.resolver.Done()
		}

		n.logger.Debug().
			Hex("node_id", logging.ID(n.id)).
			Msg("libp2p node stopped successfully")
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
	pMap := n.pCache.getPeers(pid)
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
	// certainly fail. If this Node was configured with a DHT, we can try to look up the address of
	// the peer in the DHT as a last resort.
	if len(n.host.Peerstore().Addrs(peerID)) == 0 && n.dht != nil {
		lg.Info().Msg("address not found in peer store, searching for peer in dht")

		var err error
		func() {
			timedCtx, cancel := context.WithTimeout(ctx, findPeerQueryTimeout)
			defer cancel()
			// try to find the peer using the dht
			_, err = n.dht.FindPeer(timedCtx, peerID)
		}()

		if err != nil {
			lg.Warn().Err(err).Msg("address not found in both peer store and dht")
		} else {
			lg.Debug().Msg("address not found in peer store, but found in dht search")
		}
	}
	stream, dialAddrs, err := n.unicastManager.CreateStream(ctx, peerID, maxConnectAttempt)
	if err != nil {
		return nil, flownet.NewPeerUnreachableError(fmt.Errorf("could not create stream (peer_id: %s, dialing address(s): %v): %w", peerID,
			dialAddrs, err))
	}

	lg.Debug().Str("dial_address", fmt.Sprintf("%v", dialAddrs)).Msg("stream successfully created to remote peer")
	return stream, nil
}

// GetIPPort returns the IP and Port the libp2p node is listening on.
func (n *Node) GetIPPort() (string, string, error) {
	return IPPortFromMultiAddress(n.host.Network().ListenAddresses()...)
}

// Subscribe subscribes the node to the given topic and returns the subscription
// Currently only one subscriber is allowed per topic.
// NOTE: A node will receive its own published messages.
func (n *Node) Subscribe(topic flownet.Topic, validators ...validator.MessageValidator) (*pubsub.Subscription, error) {
	n.Lock()
	defer n.Unlock()

	// Check if the topic has been already created and is in the cache
	n.pubSub.GetTopics()
	tp, found := n.topics[topic]
	var err error
	if !found {
		if n.topicValidation {
			topicValidator := validator.TopicValidator(validators...)
			if err := n.pubSub.RegisterTopicValidator(
				topic.String(), topicValidator, pubsub.WithValidatorInline(true),
			); err != nil {
				n.logger.Err(err).Str("topic", topic.String()).Msg("failed to register topic validator, aborting subscription")
				return nil, fmt.Errorf("failed to register topic validator: %w", err)
			}
		}

		tp, err = n.pubSub.Join(topic.String())
		if err != nil {
			if n.topicValidation {
				if err := n.pubSub.UnregisterTopicValidator(topic.String()); err != nil {
					n.logger.Err(err).Str("topic", topic.String()).Msg("failed to unregister topic validator")
				}
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

	if n.topicValidation {
		if err := n.pubSub.UnregisterTopicValidator(topic.String()); err != nil {
			n.logger.Err(err).Str("topic", topic.String()).Msg("failed to unregister topic validator")
		}
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
func (n *Node) Ping(ctx context.Context, peerID peer.ID) (message.PingResponse, time.Duration, error) {
	pingError := func(err error) error {
		return fmt.Errorf("failed to ping peer %s: %w", peerID, err)
	}

	targetInfo := peer.AddrInfo{ID: peerID}

	n.connMgr.Protect(targetInfo.ID, "ping")
	defer n.connMgr.Unprotect(targetInfo.ID, "ping")

	// connect to the target node
	err := n.host.Connect(ctx, targetInfo)
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
func (n *Node) UpdateAllowList(peers peer.IDSlice) {
	if n.connGater == nil {
		n.logger.Debug().Hex("node_id", logging.ID(n.id)).Msg("skipping update allow list, connection gating is not enabled")
		return
	}

	n.connGater.update(peers)
}

// Host returns pointer to host object of node.
func (n *Node) Host() host.Host {
	return n.host
}

func (n *Node) WithDefaultUnicastProtocol(defaultHandler libp2pnet.StreamHandler, preferred []unicast.ProtocolName) error {
	n.unicastManager.WithDefaultHandler(defaultHandler)
	for _, p := range preferred {
		err := n.unicastManager.Register(p)
		if err != nil {
			return fmt.Errorf("could not register unicast protocls")
		}
	}

	return nil
}

// IsConnected returns true is address is a direct peer of this node else false
func (n *Node) IsConnected(peerID peer.ID) (bool, error) {
	isConnected := n.host.Network().Connectedness(peerID) == libp2pnet.Connected
	return isConnected, nil
}
