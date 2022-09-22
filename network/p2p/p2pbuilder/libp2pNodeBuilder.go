package p2pbuilder

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/core/transport"
	discoveryRouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	"github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/network/p2p/subscription"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/network/p2p/utils"
)

// LibP2PFactoryFunc is a factory function type for generating libp2p Node instances.
type LibP2PFactoryFunc func(context.Context) (*p2pnode.Node, error)

// DefaultLibP2PNodeFactory returns a LibP2PFactoryFunc which generates the libp2p host initialized with the
// default options for the host, the pubsub and the ping service.
func DefaultLibP2PNodeFactory(
	log zerolog.Logger,
	address string,
	flowKey fcrypto.PrivateKey,
	sporkId flow.Identifier,
	idProvider module.IdentityProvider,
	metrics module.NetworkMetrics,
	resolver madns.BasicResolver,
	role string,
) LibP2PFactoryFunc {

	return func(ctx context.Context) (*p2pnode.Node, error) {
		connManager := connection.NewConnManager(log, metrics)

		// set the default connection gater peer filters for both InterceptPeerDial and InterceptSecured callbacks
		peerFilter := notEjectedPeerFilter(idProvider)
		connGater := connection.NewConnGater(log,
			connection.WithOnInterceptPeerDialFilters([]p2p.PeerFilter{peerFilter}),
			connection.WithOnInterceptSecuredFilters([]p2p.PeerFilter{peerFilter}),
		)

		builder := NewNodeBuilder(log, address, flowKey, sporkId).
			SetBasicResolver(resolver).
			SetConnectionManager(connManager).
			SetConnectionGater(connGater).
			SetRoutingSystem(func(ctx context.Context, host host.Host) (routing.Routing, error) {
				return dht.NewDHT(
					ctx,
					host,
					unicast.FlowDHTProtocolID(sporkId),
					log,
					metrics,
					dht.AsServer(),
				)
			})

		if role != "ghost" {
			r, _ := flow.ParseRole(role)
			builder.SetSubscriptionFilter(subscription.NewRoleBasedFilter(r, idProvider))
		}

		return builder.Build(ctx)
	}
}

// DefaultMessageIDFunction returns a default message ID function based on the message's data
func DefaultMessageIDFunction(msg *pb.Message) string {
	h := hash.NewSHA3_384()
	_, _ = h.Write(msg.Data)
	return h.SumHash().Hex()
}

type NodeBuilder interface {
	SetBasicResolver(madns.BasicResolver) NodeBuilder
	SetSubscriptionFilter(pubsub.SubscriptionFilter) NodeBuilder
	SetResourceManager(network.ResourceManager) NodeBuilder
	SetConnectionManager(connmgr.ConnManager) NodeBuilder
	SetConnectionGater(connmgr.ConnectionGater) NodeBuilder
	SetRoutingSystem(func(context.Context, host.Host) (routing.Routing, error)) NodeBuilder
	EnableGossipSubPeerScoring(module.IdentityProvider) NodeBuilder
	Build(context.Context) (*p2pnode.Node, error)
}

type LibP2PNodeBuilder struct {
	sporkID              flow.Identifier
	addr                 string
	networkKey           fcrypto.PrivateKey
	logger               zerolog.Logger
	basicResolver        madns.BasicResolver
	subscriptionFilter   pubsub.SubscriptionFilter
	resourceManager      network.ResourceManager
	connManager          connmgr.ConnManager
	connGater            connmgr.ConnectionGater
	idProvider           module.IdentityProvider
	gossipSubPeerScoring bool // whether to enable gossipsub peer scoring
	routingFactory       func(context.Context, host.Host) (routing.Routing, error)
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

func (builder *LibP2PNodeBuilder) SetResourceManager(manager network.ResourceManager) NodeBuilder {
	builder.resourceManager = manager
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

func (builder *LibP2PNodeBuilder) EnableGossipSubPeerScoring(provider module.IdentityProvider) NodeBuilder {
	builder.gossipSubPeerScoring = true
	builder.idProvider = provider
	return builder
}

func (builder *LibP2PNodeBuilder) Build(ctx context.Context) (*p2pnode.Node, error) {
	if builder.routingFactory == nil {
		return nil, errors.New("routing factory is not set")
	}

	var opts []libp2p.Option

	if builder.basicResolver != nil {
		resolver, err := madns.NewResolver(madns.WithDefaultResolver(builder.basicResolver))

		if err != nil {
			return nil, fmt.Errorf("could not create resolver: %w", err)
		}

		opts = append(opts, libp2p.MultiaddrResolver(resolver))
	}

	if builder.resourceManager != nil {
		opts = append(opts, libp2p.ResourceManager(builder.resourceManager))
	}

	if builder.connManager != nil {
		opts = append(opts, libp2p.ConnectionManager(builder.connManager))
	}

	if builder.connGater != nil {
		opts = append(opts, libp2p.ConnectionGater(builder.connGater))
	}

	h, err := DefaultLibP2PHost(ctx, builder.addr, builder.networkKey, opts...)
	if err != nil {
		return nil, err
	}

	rsys, err := builder.routingFactory(ctx, h)
	if err != nil {
		return nil, err
	}

	psOpts := append(
		DefaultPubsubOptions(p2pnode.DefaultMaxPubSubMsgSize),
		pubsub.WithDiscovery(discoveryRouting.NewRoutingDiscovery(rsys)),
		pubsub.WithMessageIdFn(DefaultMessageIDFunction),
	)

	if builder.subscriptionFilter != nil {
		psOpts = append(psOpts, pubsub.WithSubscriptionFilter(builder.subscriptionFilter))
	}

	var scoreOpt *scoring.ScoreOption
	if builder.gossipSubPeerScoring {
		scoreOpt = scoring.NewScoreOption(builder.logger, builder.idProvider)
		psOpts = append(psOpts, scoreOpt.BuildFlowPubSubScoreOption())
	}

	pubSub, err := pubsub.NewGossipSub(ctx, h, psOpts...)
	if err != nil {
		return nil, fmt.Errorf("could not create gossipsub: %w", err)
	}
	if scoreOpt != nil {
		scoreOpt.SetSubscriptionProvider(scoring.NewSubscriptionProvider(pubSub))
	}

	pCache, err := p2pnode.NewProtocolPeerCache(builder.logger, h)
	if err != nil {
		return nil, err
	}

	n := p2pnode.NewNode(builder.logger, h, pubSub, rsys, pCache, unicast.NewUnicastManager(
		builder.logger,
		unicast.NewLibP2PStreamFactory(h),
		builder.sporkID,
	))
	return n, nil
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

	sourceMultiAddr, err := multiaddr.NewMultiaddr(utils.MultiAddressStr(ip, port))
	if err != nil {
		return nil, fmt.Errorf("failed to translate Flow address to Libp2p multiaddress: %w", err)
	}

	// create a transport which disables port reuse and web socket.
	// Port reuse enables listening and dialing from the same TCP port (https://github.com/libp2p/go-reuseport)
	// While this sounds great, it intermittently causes a 'broken pipe' error
	// as the 1-k discovery process and the 1-1 messaging both sometimes attempt to open connection to the same target
	// As of now there is no requirement of client sockets to be a well-known port, so disabling port reuse all together.
	transport := libp2p.Transport(func(u transport.Upgrader) (*tcp.TcpTransport, error) {
		return tcp.NewTCPTransport(u, nil, tcp.DisableReuseport())
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
