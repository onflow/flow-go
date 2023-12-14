package p2pbuilder

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/go-playground/validator/v10"
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
	"github.com/onflow/flow-go/module/metrics"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/netconf"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	"github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	p2pconfig "github.com/onflow/flow-go/network/p2p/p2pbuilder/config"
	gossipsubbuilder "github.com/onflow/flow-go/network/p2p/p2pbuilder/gossipsub"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	"github.com/onflow/flow-go/network/p2p/subscription"
	"github.com/onflow/flow-go/network/p2p/tracer"
	"github.com/onflow/flow-go/network/p2p/unicast"
	unicastcache "github.com/onflow/flow-go/network/p2p/unicast/cache"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/network/p2p/unicast/stream"
	"github.com/onflow/flow-go/network/p2p/utils"
)

type DhtSystemActivation bool

const (
	DhtSystemEnabled  DhtSystemActivation = true
	DhtSystemDisabled DhtSystemActivation = false
)

type LibP2PNodeBuilder struct {
	gossipSubBuilder p2p.GossipSubBuilder
	sporkId          flow.Identifier
	address          string
	networkKey       fcrypto.PrivateKey
	logger           zerolog.Logger
	metricsConfig    *p2pconfig.MetricsConfig
	basicResolver    madns.BasicResolver

	resourceManager      network.ResourceManager
	resourceManagerCfg   *p2pconf.ResourceManagerConfig
	connManager          connmgr.ConnManager
	connGater            p2p.ConnectionGater
	routingFactory       func(context.Context, host.Host) (routing.Routing, error)
	peerManagerConfig    *p2pconfig.PeerManagerConfig
	createNode           p2p.CreateNodeFunc
	gossipSubTracer      p2p.PubSubTracer
	disallowListCacheCfg *p2p.DisallowListCacheConfig
	unicastConfig        *p2pconfig.UnicastConfig
	networkingType       flownet.NetworkingType // whether the node is running in private (staked) or public (unstaked) network
}

// LibP2PNodeBuilderConfig parameters required to create a new *LibP2PNodeBuilder with NewNodeBuilder.
type LibP2PNodeBuilderConfig struct {
	Logger                        zerolog.Logger                          `validate:"required"`
	MetricsConfig                 *p2pconfig.MetricsConfig                `validate:"required"`
	NetworkingType                flownet.NetworkingType                  `validate:"required"`
	Address                       string                                  `validate:"required"`
	NetworkKey                    fcrypto.PrivateKey                      `validate:"required"`
	SporkId                       flow.Identifier                         `validate:"required"`
	IdProvider                    module.IdentityProvider                 `validate:"required"`
	ResourceManagerParams         *p2pconf.ResourceManagerConfig          `validate:"required"`
	RpcInspectorParams            *p2pconf.GossipSubRPCInspectorsConfig   `validate:"required"`
	PeerManagerParams             *p2pconfig.PeerManagerConfig            `validate:"required"`
	SubscriptionProviderParams    *p2pconf.SubscriptionProviderParameters `validate:"required"`
	DisallowListCacheCfg          *p2p.DisallowListCacheConfig            `validate:"required"`
	UnicastParams                 *p2pconfig.UnicastConfig                `validate:"required"`
	GossipSubScorePenaltiesParams *p2pconf.GossipSubScorePenalties        `validate:"required"`
	ScoringRegistryParams         *p2pconf.GossipSubScoringRegistryConfig `validate:"required"`
}

func NewNodeBuilder(params *LibP2PNodeBuilderConfig, rpcTracking p2p.RpcControlTracking) (*LibP2PNodeBuilder, error) {
	err := validator.New().Struct(params)
	if err != nil {
		return nil, fmt.Errorf("libp2p node builder params validation failed: %w", err)
	}
	return &LibP2PNodeBuilder{
		logger:               params.Logger,
		sporkId:              params.SporkId,
		address:              params.Address,
		networkKey:           params.NetworkKey,
		createNode:           DefaultCreateNodeFunc,
		metricsConfig:        params.MetricsConfig,
		resourceManagerCfg:   params.RCfg,
		disallowListCacheCfg: params.DisallowListCacheCfg,
		networkingType:       params.NetworkingType,
		gossipSubBuilder: gossipsubbuilder.NewGossipSubBuilder(params.Logger,
			params.MetricsConfig,
			params.NetworkingType,
			params.SporkId,
			params.IdProvider,
			params.ScoringRegistryConfig,
			params.RpcInspectorCfg,
			params.SubscriptionProviderParam,
			rpcTracking,
			params.GossipSubScorePenalties,
		),
		peerManagerConfig: params.PeerManagerConfig,
		unicastConfig:     params.UnicastConfig,
	}, nil
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

func (builder *LibP2PNodeBuilder) SetGossipSubScoreTracerInterval(interval time.Duration) p2p.NodeBuilder {
	builder.gossipSubBuilder.SetGossipSubScoreTracerInterval(interval)
	return builder
}

func (builder *LibP2PNodeBuilder) OverrideDefaultRpcInspectorSuiteFactory(factory p2p.GossipSubRpcInspectorSuiteFactoryFunc) p2p.NodeBuilder {
	builder.gossipSubBuilder.OverrideDefaultRpcInspectorSuiteFactory(factory)
	return builder
}

// Build creates a new libp2p node using the configured options.
func (builder *LibP2PNodeBuilder) Build() (p2p.LibP2PNode, error) {
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
		// scales the default limits by the allowed memory and file descriptors and applies the inbound connection and stream limits.
		limits, err := BuildLibp2pResourceManagerLimits(builder.logger, builder.resourceManagerCfg)
		if err != nil {
			return nil, fmt.Errorf("could not build libp2p resource manager limits: %w", err)
		}
		mgr, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(*limits), rcmgr.WithMetrics(builder.metricsConfig.Metrics))
		if err != nil {
			return nil, fmt.Errorf("could not create libp2p resource manager: %w", err)
		}

		opts = append(opts, libp2p.ResourceManager(mgr))
		builder.logger.Info().Msg("default libp2p resource manager is enabled with metrics")
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
	lg := builder.logger.With().Str("local_peer_id", p2plogging.PeerId(h.ID())).Logger()

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
		peerUpdater, err := connection.NewPeerUpdater(
			&connection.PeerUpdaterConfig{
				PruneConnections: builder.peerManagerConfig.ConnectionPruning,
				Logger:           lg,
				Host:             connection.NewConnectorHost(h),
				Connector:        connector,
			})
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2p connector: %w", err)
		}

		peerManager = connection.NewPeerManager(lg, builder.peerManagerConfig.UpdateInterval, peerUpdater)

		if builder.unicastConfig.RateLimiterDistributor != nil {
			builder.unicastConfig.RateLimiterDistributor.AddConsumer(peerManager)
		}
	}

	node := builder.createNode(lg, h, pCache, peerManager, builder.disallowListCacheCfg)

	if builder.connGater != nil {
		builder.connGater.SetDisallowListOracle(node)
	}

	unicastManager, err := unicast.NewUnicastManager(&unicast.ManagerConfig{
		Logger:                             lg,
		StreamFactory:                      stream.NewLibP2PStreamFactory(h),
		SporkId:                            builder.sporkId,
		CreateStreamBackoffDelay:           builder.unicastConfig.CreateStreamBackoffDelay,
		Metrics:                            builder.metricsConfig.Metrics,
		StreamZeroRetryResetThreshold:      builder.unicastConfig.StreamZeroRetryResetThreshold,
		MaxStreamCreationRetryAttemptTimes: builder.unicastConfig.MaxStreamCreationRetryAttemptTimes,
		UnicastConfigCacheFactory: func(configFactory func() unicast.Config) unicast.ConfigCache {
			return unicastcache.NewUnicastConfigCache(builder.unicastConfig.ConfigCacheSize,
				lg,
				metrics.DialConfigCacheMetricFactory(builder.metricsConfig.HeroCacheFactory, builder.networkingType),
				configFactory)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not create unicast manager: %w", err)
	}
	node.SetUnicastManager(unicastManager)

	cm := component.NewComponentManagerBuilder().
		AddWorker(
			func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
				if builder.routingFactory != nil {
					routingSystem, err := builder.routingFactory(ctx, h)
					if err != nil {
						ctx.Throw(fmt.Errorf("could not create routing system: %w", err))
					}
					if err := node.SetRouting(routingSystem); err != nil {
						ctx.Throw(fmt.Errorf("could not set routing system: %w", err))
					}
					builder.gossipSubBuilder.SetRoutingSystem(routingSystem)
					lg.Debug().Msg("routing system created")
				}
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
		AddWorker(
			func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
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
	t := libp2p.Transport(
		func(u transport.Upgrader) (*tcp.TcpTransport, error) {
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
func DefaultCreateNodeFunc(
	logger zerolog.Logger,
	host host.Host,
	pCache p2p.ProtocolPeerCache,
	peerManager p2p.PeerManager,
	disallowListCacheCfg *p2p.DisallowListCacheConfig,
) p2p.LibP2PNode {
	return p2pnode.NewNode(logger, host, pCache, peerManager, disallowListCacheCfg)
}

// DefaultNodeBuilder returns a node builder.
func DefaultNodeBuilder(params *LibP2PNodeBuilderConfig,
	resolver madns.BasicResolver,
	role string,
	connGaterCfg *p2pconfig.ConnectionGaterConfig,
	gossipCfg *p2pconf.GossipSubConfig,
	connMgrConfig *netconf.ConnectionManagerConfig,
	dhtSystemActivation DhtSystemActivation,
) (p2p.NodeBuilder, error) {

	connManager, err := connection.NewConnManager(params.Logger, params.MetricsConfig.Metrics, connMgrConfig)
	if err != nil {
		return nil, fmt.Errorf("could not create connection manager: %w", err)
	}

	// set the default connection gater peer filters for both InterceptPeerDial and InterceptSecured callbacks
	peerFilter := notEjectedPeerFilter(params.IdProvider)
	peerFilters := []p2p.PeerFilter{peerFilter}

	connGater := connection.NewConnGater(
		params.Logger,
		params.IdProvider,
		connection.WithOnInterceptPeerDialFilters(append(peerFilters, connGaterCfg.InterceptPeerDialFilters...)),
		connection.WithOnInterceptSecuredFilters(append(peerFilters, connGaterCfg.InterceptSecuredFilters...)))

	meshTracerCfg := &tracer.GossipSubMeshTracerConfig{
		Logger:                             params.Logger,
		Metrics:                            params.MetricsConfig.Metrics,
		IDProvider:                         params.IdProvider,
		LoggerInterval:                     gossipCfg.LocalMeshLogInterval,
		RpcSentTrackerCacheSize:            gossipCfg.RPCSentTrackerCacheSize,
		RpcSentTrackerWorkerQueueCacheSize: gossipCfg.RPCSentTrackerQueueCacheSize,
		RpcSentTrackerNumOfWorkers:         gossipCfg.RpcSentTrackerNumOfWorkers,
		HeroCacheMetricsFactory:            params.MetricsConfig.HeroCacheFactory,
		NetworkingType:                     flownet.PrivateNetwork,
	}
	meshTracer := tracer.NewGossipSubMeshTracer(meshTracerCfg)

	builder, err := NewNodeBuilder(params, meshTracer)
	if err != nil {
		return nil, fmt.Errorf("could not create libp2p node builder: %w", err)
	}
	builder.
		SetBasicResolver(resolver).
		SetConnectionManager(connManager).
		SetConnectionGater(connGater).
		SetCreateNode(DefaultCreateNodeFunc)

	if gossipCfg.PeerScoring {
		// In production, we never override the default scoring config.
		builder.EnableGossipSubScoringWithOverride(p2p.PeerScoringConfigNoOverride)
	}

	builder.SetGossipSubTracer(meshTracer)
	builder.SetGossipSubScoreTracerInterval(gossipCfg.ScoreTracerInterval)

	if role != "ghost" {
		r, err := flow.ParseRole(role)
		if err != nil {
			return nil, fmt.Errorf("could not parse role: %w", err)
		}
		builder.SetSubscriptionFilter(subscription.NewRoleBasedFilter(r, params.IdProvider))

		if dhtSystemActivation == DhtSystemEnabled {
			builder.SetRoutingSystem(
				func(ctx context.Context, host host.Host) (routing.Routing, error) {
					return dht.NewDHT(ctx, host, protocols.FlowDHTProtocolID(params.SporkId), params.Logger, params.MetricsConfig.Metrics, dht.AsServer())
				})
		}
	}

	return builder, nil
}
