package p2p

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	stream "github.com/libp2p/go-libp2p-transport-upgrader"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p/dns"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/p2p/unicast"
)

// LibP2PFactoryFunc is a factory function type for generating libp2p Node instances.
type LibP2PFactoryFunc func(context.Context) (*Node, error)

// DefaultLibP2PNodeFactory returns a LibP2PFactoryFunc which generates the libp2p host initialized with the
// default options for the host, the pubsub and the ping service.
func DefaultLibP2PNodeFactory(
	log zerolog.Logger,
	address string,
	flowKey fcrypto.PrivateKey,
	sporkId flow.Identifier,
	idProvider id.IdentityProvider,
	metrics module.NetworkMetrics,
	dnsResolverTTL time.Duration,
	role string,
) LibP2PFactoryFunc {

	return func(ctx context.Context) (*Node, error) {
		connManager := NewConnManager(log, metrics)
		connGater := NewConnGater(log, func(pid peer.ID) bool {
			_, found := idProvider.ByPeerID(pid)

			return found
		})
		resolver := dns.NewResolver(metrics, dns.WithTTL(dnsResolverTTL))

		builder := NewNodeBuilder(log, address, flowKey, sporkId).
			SetBasicResolver(resolver).
			SetConnectionManager(connManager).
			SetConnectionGater(connGater).
			SetRoutingSystem(func(ctx context.Context, host host.Host) (routing.Routing, error) {
				return NewDHT(ctx, host, unicast.FlowDHTProtocolID(sporkId), AsServer(true))
			}).
			SetPubSub(pubsub.NewGossipSub)

		if role != "ghost" {
			r, _ := flow.ParseRole(role)
			builder.SetSubscriptionFilter(NewRoleBasedFilter(r, idProvider))
		}

		return builder.Build(ctx)
	}
}

type NodeBuilder interface {
	SetBasicResolver(madns.BasicResolver) NodeBuilder
	SetSubscriptionFilter(pubsub.SubscriptionFilter) NodeBuilder
	SetConnectionManager(connmgr.ConnManager) NodeBuilder
	SetConnectionGater(connmgr.ConnectionGater) NodeBuilder
	SetRoutingSystem(func(context.Context, host.Host) (routing.Routing, error)) NodeBuilder
	SetPubSub(func(context.Context, host.Host, ...pubsub.Option) (*pubsub.PubSub, error)) NodeBuilder
	Build(context.Context) (*Node, error)
}

type LibP2PNodeBuilder struct {
	sporkID            flow.Identifier
	addr               string
	networkKey         fcrypto.PrivateKey
	logger             zerolog.Logger
	basicResolver      madns.BasicResolver
	subscriptionFilter pubsub.SubscriptionFilter
	connManager        connmgr.ConnManager
	connGater          connmgr.ConnectionGater
	routingFactory     func(context.Context, host.Host) (routing.Routing, error)
	pubsubFactory      func(context.Context, host.Host, ...pubsub.Option) (*pubsub.PubSub, error)
}

func NewNodeBuilder(
	logger zerolog.Logger,
	addr string,
	networkKey fcrypto.PrivateKey,
	sporkID flow.Identifier,
) *LibP2PNodeBuilder {
	return &LibP2PNodeBuilder{
		logger:     logger,
		sporkID:    sporkID,
		addr:       addr,
		networkKey: networkKey,
	}
}

func (builder *LibP2PNodeBuilder) SetBasicResolver(br madns.BasicResolver) NodeBuilder {
	builder.basicResolver = br
	return builder
}

func (builder *LibP2PNodeBuilder) SetSubscriptionFilter(filter pubsub.SubscriptionFilter) NodeBuilder {
	builder.subscriptionFilter = filter
	return builder
}

func (builder *LibP2PNodeBuilder) SetConnectionManager(manager connmgr.ConnManager) NodeBuilder {
	builder.connManager = manager
	return builder
}

func (builder *LibP2PNodeBuilder) SetConnectionGater(gater connmgr.ConnectionGater) NodeBuilder {
	builder.connGater = gater
	return builder
}

func (builder *LibP2PNodeBuilder) SetRoutingSystem(f func(context.Context, host.Host) (routing.Routing, error)) NodeBuilder {
	builder.routingFactory = f
	return builder
}

func (builder *LibP2PNodeBuilder) SetPubSub(f func(context.Context, host.Host, ...pubsub.Option) (*pubsub.PubSub, error)) NodeBuilder {
	builder.pubsubFactory = f
	return builder
}

func (builder *LibP2PNodeBuilder) Build(ctx context.Context) (*Node, error) {
	if builder.routingFactory == nil {
		return nil, errors.New("routing factory is not set")
	}

	if builder.pubsubFactory == nil {
		return nil, errors.New("pubsub factory is not set")
	}

	var opts []libp2p.Option

	if builder.basicResolver != nil {
		resolver, err := madns.NewResolver(madns.WithDefaultResolver(builder.basicResolver))

		if err != nil {
			return nil, fmt.Errorf("could not create resolver: %w", err)
		}

		opts = append(opts, libp2p.MultiaddrResolver(resolver))
	}

	if builder.connManager != nil {
		opts = append(opts, libp2p.ConnectionManager(builder.connManager))
	}

	if builder.connGater != nil {
		opts = append(opts, libp2p.ConnectionGater(builder.connGater))
	}

	host, err := DefaultLibP2PHost(ctx, builder.addr, builder.networkKey, opts...)

	if err != nil {
		return nil, err
	}

	rsys, err := builder.routingFactory(ctx, host)

	if err != nil {
		return nil, err
	}

	psOpts := append(
		DefaultPubsubOptions(DefaultMaxPubSubMsgSize),
		pubsub.WithDiscovery(discovery.NewRoutingDiscovery(rsys)),
	)

	if builder.subscriptionFilter != nil {
		psOpts = append(psOpts, pubsub.WithSubscriptionFilter(builder.subscriptionFilter))
	}

	pubSub, err := builder.pubsubFactory(ctx, host, psOpts...)

	if err != nil {
		return nil, err
	}

	pCache, err := newProtocolPeerCache(builder.logger, host)

	if err != nil {
		return nil, err
	}

	node := &Node{
		topics:  make(map[flownet.Topic]*pubsub.Topic),
		subs:    make(map[flownet.Topic]*pubsub.Subscription),
		logger:  builder.logger,
		routing: rsys,
		host:    host,
		unicastManager: unicast.NewUnicastManager(
			builder.logger,
			unicast.NewLibP2PStreamFactory(host),
			builder.sporkID,
		),
		pCache: pCache,
		pubSub: pubSub,
	}

	return node, nil
}

// DefaultLibP2PHost returns a libp2p host initialized to listen on the given address and using the given private key and
// customized with options
func DefaultLibP2PHost(ctx context.Context, address string, key fcrypto.PrivateKey, options ...config.Option) (host.Host,
	error) {
	defaultOptions, err := defaultLibP2POptions(address, key)
	if err != nil {
		return nil, err
	}

	allOptions := append(defaultOptions, options...)

	// create the libp2p host
	libP2PHost, err := libp2p.New(allOptions...)
	if err != nil {
		return nil, fmt.Errorf("could not create libp2p host: %w", err)
	}

	return libP2PHost, nil
}

// defaultLibP2POptions creates and returns the standard LibP2P host options that are used for the Flow Libp2p network
func defaultLibP2POptions(address string, key fcrypto.PrivateKey) ([]config.Option, error) {

	libp2pKey, err := keyutils.LibP2PPrivKeyFromFlow(key)
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
	transport := libp2p.Transport(func(u *stream.Upgrader) (*tcp.TcpTransport, error) {
		return tcp.NewTCPTransport(u, tcp.DisableReuseport())
	})

	// gather all the options for the libp2p node
	options := []config.Option{
		libp2p.ListenAddrs(sourceMultiAddr), // set the listen address
		libp2p.Identity(libp2pKey),          // pass in the networking key
		transport,                           // set the protocol
	}

	return options, nil
}

func DefaultPubsubOptions(maxPubSubMsgSize int) []pubsub.Option {
	return []pubsub.Option{
		// skip message signing
		pubsub.WithMessageSigning(true),
		// skip message signature
		pubsub.WithStrictSignatureVerification(true),
		// set max message size limit for 1-k PubSub messaging
		pubsub.WithMaxMessageSize(maxPubSubMsgSize),
		// no discovery
	}
}
