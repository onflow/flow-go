package p2pbuilder

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/core/transport"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/netconf"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	"github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	p2pconfig "github.com/onflow/flow-go/network/p2p/p2pbuilder/config"
	gossipsubbuilder "github.com/onflow/flow-go/network/p2p/p2pbuilder/gossipsub"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	"github.com/onflow/flow-go/network/p2p/subscription"
	"github.com/onflow/flow-go/network/p2p/tracer"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/network/p2p/unicast/stream"
	"github.com/onflow/flow-go/network/p2p/utils"
)

type GossipSubFactoryFunc func(context.Context, zerolog.Logger, host.Host, p2p.PubSubAdapterConfig) (p2p.PubSubAdapter, error)
type CreateNodeFunc func(logger zerolog.Logger,
	host host.Host,
	pCache *p2pnode.ProtocolPeerCache,
	peerManager *connection.PeerManager) p2p.LibP2PNode
type GossipSubAdapterConfigFunc func(*p2p.BasePubSubAdapterConfig) p2p.PubSubAdapterConfig

type LibP2PNodeBuilder struct {
	gossipSubBuilder p2p.GossipSubBuilder
	sporkId          flow.Identifier
	address          string
	networkKey       fcrypto.PrivateKey
	logger           zerolog.Logger
	metrics          module.LibP2PMetrics
	basicResolver    madns.BasicResolver

	resourceManager           network.ResourceManager
	resourceManagerCfg        *p2pconf.ResourceManagerConfig
	connManager               connmgr.ConnManager
	connGater                 p2p.ConnectionGater
	routingFactory            func(context.Context, host.Host) (routing.Routing, error)
	peerManagerConfig         *p2pconfig.PeerManagerConfig
	createNode                p2p.CreateNodeFunc
	createStreamRetryInterval time.Duration
	rateLimiterDistributor    p2p.UnicastRateLimiterDistributor
	gossipSubTracer           p2p.PubSubTracer
	disallowListCacheCfg      *p2p.DisallowListCacheConfig
}

func NewNodeBuilder(
	logger zerolog.Logger,
	metricsConfig *p2pconfig.MetricsConfig,
	networkingType flownet.NetworkingType,
	address string,
	networkKey fcrypto.PrivateKey,
	sporkId flow.Identifier,
	idProvider module.IdentityProvider,
	rCfg *p2pconf.ResourceManagerConfig,
	rpcInspectorCfg *p2pconf.GossipSubRPCInspectorsConfig,
	peerManagerConfig *p2pconfig.PeerManagerConfig,
	disallowListCacheCfg *p2p.DisallowListCacheConfig) *LibP2PNodeBuilder {
	return &LibP2PNodeBuilder{
		logger:               logger,
		sporkId:              sporkId,
		address:              address,
		networkKey:           networkKey,
		createNode:           DefaultCreateNodeFunc,
		metrics:              metricsConfig.Metrics,
		resourceManagerCfg:   rCfg,
		disallowListCacheCfg: disallowListCacheCfg,
		gossipSubBuilder: gossipsubbuilder.NewGossipSubBuilder(
			logger,
			metricsConfig,
			networkingType,
			sporkId,
			idProvider,
			rpcInspectorCfg),
		peerManagerConfig: peerManagerConfig,
	}
}

var _ p2p.NodeBuilder = &LibP2PNodeBuilder{}

// SetBasicResolver sets the DNS resolver for the node.
func (builder *LibP2PNodeBuilder) SetBasicResolver(br madns.BasicResolver) p2p.NodeBuilder {
	builder.basicResolver = br
	return builder
}

// SetSubscriptionFilter sets the pubsub subscription filter for the node.
func (builder *LibP2PNodeBuilder) SetSubscriptionFilter(filter pubsub.SubscriptionFilter) p2p.NodeBuilder {
	builder.gossipSubBuilder.SetSubscriptionFilter(filter)
	return builder
}

// SetResourceManager sets the resource manager for the node.
func (builder *LibP2PNodeBuilder) SetResourceManager(manager network.ResourceManager) p2p.NodeBuilder {
	builder.resourceManager = manager
	return builder
}

// SetConnectionManager sets the connection manager for the node.
func (builder *LibP2PNodeBuilder) SetConnectionManager(manager connmgr.ConnManager) p2p.NodeBuilder {
	builder.connManager = manager
	return builder
}

// SetConnectionGater sets the connection gater for the node.
func (builder *LibP2PNodeBuilder) SetConnectionGater(gater p2p.ConnectionGater) p2p.NodeBuilder {
	builder.connGater = gater
	return builder
}

// SetRoutingSystem sets the routing system factory function.
func (builder *LibP2PNodeBuilder) SetRoutingSystem(f func(context.Context, host.Host) (routing.Routing, error)) p2p.NodeBuilder {
	builder.routingFactory = f
	return builder
}

func (builder *LibP2PNodeBuilder) SetGossipSubFactory(gf p2p.GossipSubFactoryFunc, cf p2p.GossipSubAdapterConfigFunc) p2p.NodeBuilder {
	builder.gossipSubBuilder.SetGossipSubFactory(gf)
	builder.gossipSubBuilder.SetGossipSubConfigFunc(cf)
	return builder
}

// EnableGossipSubScoringWithOverride enables peer scoring for the GossipSub pubsub system with the given override.
// Any existing peer scoring config attribute that is set in the override will override the default peer scoring config.
// Anything that is left to nil or zero value in the override will be ignored and the default value will be used.
// Note: it is not recommended to override the default peer scoring config in production unless you know what you are doing.
// Production Tip: use PeerScoringConfigNoOverride as the argument to this function to enable peer scoring without any override.
// Args:
// - PeerScoringConfigOverride: override for the peer scoring config- Recommended to use PeerScoringConfigNoOverride for production.
// Returns:
// none
func (builder *LibP2PNodeBuilder) EnableGossipSubScoringWithOverride(config *p2p.PeerScoringConfigOverride) p2p.NodeBuilder {
	builder.gossipSubBuilder.EnableGossipSubScoringWithOverride(config)
	return builder
}

func (builder *LibP2PNodeBuilder) SetGossipSubTracer(tracer p2p.PubSubTracer) p2p.NodeBuilder {
	builder.gossipSubBuilder.SetGossipSubTracer(tracer)
	builder.gossipSubTracer = tracer
	return builder
}

func (builder *LibP2PNodeBuilder) SetCreateNode(f p2p.CreateNodeFunc) p2p.NodeBuilder {
	builder.createNode = f
	return builder
}

func (builder *LibP2PNodeBuilder) SetRateLimiterDistributor(distributor p2p.UnicastRateLimiterDistributor) p2p.NodeBuilder {
	builder.rateLimiterDistributor = distributor
	return builder
}

func (builder *LibP2PNodeBuilder) SetStreamCreationRetryInterval(createStreamRetryInterval time.Duration) p2p.NodeBuilder {
	builder.createStreamRetryInterval = createStreamRetryInterval
	return builder
}

func (builder *LibP2PNodeBuilder) SetGossipSubScoreTracerInterval(interval time.Duration) p2p.NodeBuilder {
	builder.gossipSubBuilder.SetGossipSubScoreTracerInterval(interval)
	return builder
}

func (builder *LibP2PNodeBuilder) OverrideDefaultRpcInspectorSuiteFactory(factory p2p.GossipSubRpcInspectorSuiteFactoryFunc) p2p.NodeBuilder {
	builder.gossipSubBuilder.OverrideDefaultRpcInspectorSuiteFactory(factory)
	return builder
}

// buildRouting creates a new routing system factory for a libp2p node using the provided host.
// It returns the newly created routing system and any errors encountered during its creation.
//
// Arguments:
// - ctx: a context.Context object used to manage the lifecycle of the node.
// - h: a libp2p host.Host object used to initialize the routing system.
//
// Returns:
// - routing.Routing: a routing system for the libp2p node.
// - error: if an error occurs during the creation of the routing system, it is returned. Otherwise, nil is returned.
// Note that on happy path, the returned error is nil. Any non-nil error indicates that the routing system could not be created
// and is non-recoverable. In case of an error the node should be stopped.
func (builder *LibP2PNodeBuilder) buildRouting(ctx context.Context, h host.Host) (routing.Routing, error) {
	routingSystem, err := builder.routingFactory(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("could not create libp2p node routing system: %w", err)
	}
	return routingSystem, nil
}

// Build creates a new libp2p node using the configured options.
func (builder *LibP2PNodeBuilder) Build() (p2p.LibP2PNode, error) {
	if builder.routingFactory == nil {
		return nil, errors.New("routing system factory is not set")
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
		builder.logger.Warn().
			Msg("libp2p resource manager is overridden by the node builder, metrics may not be available")
	} else {
		// setting up default resource manager, by hooking in the resource manager metrics reporter.
		limits := rcmgr.DefaultLimits
		libp2p.SetDefaultServiceLimits(&limits)

		mem, err := allowedMemory(builder.resourceManagerCfg.MemoryLimitRatio)
		if err != nil {
			return nil, fmt.Errorf("could not get allowed memory: %w", err)
		}
		fd, err := allowedFileDescriptors(builder.resourceManagerCfg.FileDescriptorsRatio)
		if err != nil {
			return nil, fmt.Errorf("could not get allowed file descriptors: %w", err)
		}
		limits.PeerBaseLimit.ConnsInbound = builder.resourceManagerCfg.PeerBaseLimitConnsInbound
		l := limits.Scale(mem, fd)
		mgr, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(l), rcmgr.WithMetrics(builder.metrics))
		if err != nil {
			return nil, fmt.Errorf("could not create libp2p resource manager: %w", err)
		}
		builder.logger.Info().
			Str("key", keyResourceManagerLimit).
			Int64("allowed_memory", mem).
			Int("allowed_file_descriptors", fd).
			Msg("allowed memory and file descriptors are fetched from the system")
		newLimitConfigLogger(builder.logger).logResourceManagerLimits(l)

		opts = append(opts, libp2p.ResourceManager(mgr))
		builder.logger.Info().Msg("libp2p resource manager is set to default with metrics")
	}

	if builder.connManager != nil {
		opts = append(opts, libp2p.ConnectionManager(builder.connManager))
	}

	if builder.connGater != nil {
		opts = append(opts, libp2p.ConnectionGater(builder.connGater))
	}

	h, err := DefaultLibP2PHost(builder.address, builder.networkKey, opts...)
	if err != nil {
		return nil, err
	}
	builder.gossipSubBuilder.SetHost(h)

	pCache, err := p2pnode.NewProtocolPeerCache(builder.logger, h)
	if err != nil {
		return nil, err
	}

	var peerManager p2p.PeerManager
	if builder.peerManagerConfig.UpdateInterval > 0 {
		connector, err := builder.peerManagerConfig.ConnectorFactory(h)
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2p connector: %w", err)
		}
		peerUpdater, err := connection.NewPeerUpdater(&connection.PeerUpdaterConfig{
			PruneConnections: builder.peerManagerConfig.ConnectionPruning,
			Logger:           builder.logger,
			Host:             connection.NewConnectorHost(h),
			Connector:        connector,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2p connector: %w", err)
		}

		peerManager = connection.NewPeerManager(builder.logger, builder.peerManagerConfig.UpdateInterval, peerUpdater)

		if builder.rateLimiterDistributor != nil {
			builder.rateLimiterDistributor.AddConsumer(peerManager)
		}
	}

	node := builder.createNode(builder.logger, h, pCache, peerManager, builder.disallowListCacheCfg)

	if builder.connGater != nil {
		builder.connGater.SetDisallowListOracle(node)
	}

	unicastManager := unicast.NewUnicastManager(builder.logger,
		stream.NewLibP2PStreamFactory(h),
		builder.sporkId,
		builder.createStreamRetryInterval,
		node,
		builder.metrics)
	node.SetUnicastManager(unicastManager)

	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			// routing system is created here, because it needs to be created during the node startup.
			routingSystem, err := builder.buildRouting(ctx, h)
			if err != nil {
				ctx.Throw(fmt.Errorf("could not create routing system: %w", err))
			}
			node.SetRouting(routingSystem)
			builder.gossipSubBuilder.SetRoutingSystem(routingSystem)

			// gossipsub is created here, because it needs to be created during the node startup.
			gossipSub, err := builder.gossipSubBuilder.Build(ctx)
			if err != nil {
				ctx.Throw(fmt.Errorf("could not create gossipsub: %w", err))
			}
			node.SetPubSub(gossipSub)
			gossipSub.Start(ctx)
			ready()

			<-gossipSub.Done()
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			// encapsulates shutdown logic for the libp2p node.
			ready()
			<-ctx.Done()
			// we wait till the context is done, and then we stop the libp2p node.

			err = node.Stop()
			if err != nil {
				// ignore context cancellation errors
				if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					ctx.Throw(fmt.Errorf("could not stop libp2p node: %w", err))
				}
			}
		})

	node.SetComponentManager(cm.Build())

	return node, nil
}

// DefaultLibP2PHost returns a libp2p host initialized to listen on the given address and using the given private key and
// customized with options
func DefaultLibP2PHost(address string, key fcrypto.PrivateKey, options ...config.Option) (host.Host, error) {
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
	t := libp2p.Transport(func(u transport.Upgrader) (*tcp.TcpTransport, error) {
		return tcp.NewTCPTransport(u, nil, tcp.DisableReuseport())
	})

	// gather all the options for the libp2p node
	options := []config.Option{
		libp2p.ListenAddrs(sourceMultiAddr), // set the listen address
		libp2p.Identity(libp2pKey),          // pass in the networking key
		t,                                   // set the transport
	}

	return options, nil
}

// DefaultCreateNodeFunc returns new libP2P node.
func DefaultCreateNodeFunc(logger zerolog.Logger,
	host host.Host,
	pCache p2p.ProtocolPeerCache,
	peerManager p2p.PeerManager,
	disallowListCacheCfg *p2p.DisallowListCacheConfig) p2p.LibP2PNode {
	return p2pnode.NewNode(logger, host, pCache, peerManager, disallowListCacheCfg)
}

// DefaultNodeBuilder returns a node builder.
func DefaultNodeBuilder(
	logger zerolog.Logger,
	address string,
	networkingType flownet.NetworkingType,
	flowKey fcrypto.PrivateKey,
	sporkId flow.Identifier,
	idProvider module.IdentityProvider,
	metricsCfg *p2pconfig.MetricsConfig,
	resolver madns.BasicResolver,
	role string,
	connGaterCfg *p2pconfig.ConnectionGaterConfig,
	peerManagerCfg *p2pconfig.PeerManagerConfig,
	gossipCfg *p2pconf.GossipSubConfig,
	rpcInspectorCfg *p2pconf.GossipSubRPCInspectorsConfig,
	rCfg *p2pconf.ResourceManagerConfig,
	uniCfg *p2pconfig.UnicastConfig,
	connMgrConfig *netconf.ConnectionManagerConfig,
	disallowListCacheCfg *p2p.DisallowListCacheConfig) (p2p.NodeBuilder, error) {

	connManager, err := connection.NewConnManager(logger, metricsCfg.Metrics, connMgrConfig)
	if err != nil {
		return nil, fmt.Errorf("could not create connection manager: %w", err)
	}

	// set the default connection gater peer filters for both InterceptPeerDial and InterceptSecured callbacks
	peerFilter := notEjectedPeerFilter(idProvider)
	peerFilters := []p2p.PeerFilter{peerFilter}

	connGater := connection.NewConnGater(logger,
		idProvider,
		connection.WithOnInterceptPeerDialFilters(append(peerFilters, connGaterCfg.InterceptPeerDialFilters...)),
		connection.WithOnInterceptSecuredFilters(append(peerFilters, connGaterCfg.InterceptSecuredFilters...)))

	builder := NewNodeBuilder(
		logger,
		metricsCfg,
		networkingType,
		address,
		flowKey,
		sporkId,
		idProvider,
		rCfg,
		rpcInspectorCfg,
		peerManagerCfg,
		disallowListCacheCfg).
		SetBasicResolver(resolver).
		SetConnectionManager(connManager).
		SetConnectionGater(connGater).
		SetRoutingSystem(func(ctx context.Context, host host.Host) (routing.Routing, error) {
			return dht.NewDHT(ctx, host, protocols.FlowDHTProtocolID(sporkId), logger, metricsCfg.Metrics, dht.AsServer())
		}).
		SetStreamCreationRetryInterval(uniCfg.StreamRetryInterval).
		SetCreateNode(DefaultCreateNodeFunc).
		SetRateLimiterDistributor(uniCfg.RateLimiterDistributor)

	if gossipCfg.PeerScoring {
		// In production, we never override the default scoring config.
		builder.EnableGossipSubScoringWithOverride(p2p.PeerScoringConfigNoOverride)
	}

	meshTracerCfg := &tracer.GossipSubMeshTracerConfig{
		Logger:                             logger,
		Metrics:                            metricsCfg.Metrics,
		IDProvider:                         idProvider,
		LoggerInterval:                     gossipCfg.LocalMeshLogInterval,
		RpcSentTrackerCacheSize:            gossipCfg.RPCSentTrackerCacheSize,
		RpcSentTrackerWorkerQueueCacheSize: gossipCfg.RPCSentTrackerQueueCacheSize,
		RpcSentTrackerNumOfWorkers:         gossipCfg.RpcSentTrackerNumOfWorkers,
		HeroCacheMetricsFactory:            metricsCfg.HeroCacheFactory,
		NetworkingType:                     flownet.PrivateNetwork,
	}
	meshTracer := tracer.NewGossipSubMeshTracer(meshTracerCfg)

	builder.SetGossipSubTracer(meshTracer)
	builder.SetGossipSubScoreTracerInterval(gossipCfg.ScoreTracerInterval)

	if role != "ghost" {
		r, _ := flow.ParseRole(role)
		builder.SetSubscriptionFilter(subscription.NewRoleBasedFilter(r, idProvider))
	}

	return builder, nil
}
