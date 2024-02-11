package p2pbuilder

import (
	"context"
	"errors"
	"fmt"
	"net"

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
	fcrypto "github.com/onflow/crypto"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/netconf"
	"github.com/onflow/flow-go/network/p2p"
	p2pbuilderconfig "github.com/onflow/flow-go/network/p2p/builder/config"
	gossipsubbuilder "github.com/onflow/flow-go/network/p2p/builder/gossipsub"
	p2pconfig "github.com/onflow/flow-go/network/p2p/config"
	"github.com/onflow/flow-go/network/p2p/connection"
	"github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
	p2pnode "github.com/onflow/flow-go/network/p2p/node"
	"github.com/onflow/flow-go/network/p2p/subscription"
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
	metricsConfig    *p2pbuilderconfig.MetricsConfig
	basicResolver    madns.BasicResolver

	resourceManager      network.ResourceManager
	resourceManagerCfg   *p2pconfig.ResourceManagerConfig
	connManager          connmgr.ConnManager
	connGater            p2p.ConnectionGater
	routingFactory       func(context.Context, host.Host) (routing.Routing, error)
	peerManagerConfig    *p2pbuilderconfig.PeerManagerConfig
	createNode           p2p.NodeConstructor
	disallowListCacheCfg *p2p.DisallowListCacheConfig
	unicastConfig        *p2pbuilderconfig.UnicastConfig
	networkingType       flownet.NetworkingType // whether the node is running in private (staked) or public (unstaked) network
}

func NewNodeBuilder(
	logger zerolog.Logger,
	gossipSubCfg *p2pconfig.GossipSubParameters,
	metricsConfig *p2pbuilderconfig.MetricsConfig,
	networkingType flownet.NetworkingType,
	address string,
	networkKey fcrypto.PrivateKey,
	sporkId flow.Identifier,
	idProvider module.IdentityProvider,
	rCfg *p2pconfig.ResourceManagerConfig,
	peerManagerConfig *p2pbuilderconfig.PeerManagerConfig,
	disallowListCacheCfg *p2p.DisallowListCacheConfig,
	unicastConfig *p2pbuilderconfig.UnicastConfig,
) *LibP2PNodeBuilder {
	return &LibP2PNodeBuilder{
		logger:               logger,
		sporkId:              sporkId,
		address:              address,
		networkKey:           networkKey,
		createNode:           func(cfg *p2p.NodeConfig) (p2p.LibP2PNode, error) { return p2pnode.NewNode(cfg) },
		metricsConfig:        metricsConfig,
		resourceManagerCfg:   rCfg,
		disallowListCacheCfg: disallowListCacheCfg,
		networkingType:       networkingType,
		gossipSubBuilder: gossipsubbuilder.NewGossipSubBuilder(logger,
			metricsConfig,
			gossipSubCfg,
			networkingType,
			sporkId,
			idProvider),
		peerManagerConfig: peerManagerConfig,
		unicastConfig:     unicastConfig,
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

// OverrideGossipSubFactory overrides the default gossipsub factory for the GossipSub protocol.
// The purpose of override is to allow the node to provide a custom gossipsub factory for sake of testing or experimentation.
// Note: it is not recommended to override the default gossipsub factory in production unless you know what you are doing.
// Args:
// - factory: custom gossipsub factory
// Returns:
// - NodeBuilder: the node builder
func (builder *LibP2PNodeBuilder) OverrideGossipSubFactory(gf p2p.GossipSubFactoryFunc, cf p2p.GossipSubAdapterConfigFunc) p2p.NodeBuilder {
	builder.gossipSubBuilder.SetGossipSubFactory(gf)
	builder.gossipSubBuilder.SetGossipSubConfigFunc(cf)
	return builder
}

// OverrideGossipSubScoringConfig overrides the default peer scoring config for the GossipSub protocol.
// Note that it does not enable peer scoring. The peer scoring is enabled directly by setting the `peer-scoring-enabled` flag to true in `default-config.yaml`, or
// by setting the `gossipsub-peer-scoring-enabled` runtime flag to true. This function only overrides the default peer scoring config which takes effect
// only if the peer scoring is enabled (mostly for testing purposes).
// Any existing peer scoring config attribute that is set in the override will override the default peer scoring config.
// Anything that is left to nil or zero value in the override will be ignored and the default value will be used.
// Note: it is not recommended to override the default peer scoring config in production unless you know what you are doing.
// Args:
// - PeerScoringConfigOverride: override for the peer scoring config- Recommended to use PeerScoringConfigNoOverride for production.
// Returns:
// none
func (builder *LibP2PNodeBuilder) OverrideGossipSubScoringConfig(config *p2p.PeerScoringConfigOverride) p2p.NodeBuilder {
	builder.gossipSubBuilder.EnableGossipSubScoringWithOverride(config)
	return builder
}

// OverrideNodeConstructor overrides the default node constructor, i.e., the function that creates a new libp2p node.
// The purpose of override is to allow the node to provide a custom node constructor for sake of testing or experimentation.
// It is NOT recommended to override the default node constructor in production unless you know what you are doing.
// Args:
// - NodeConstructor: custom node constructor
// Returns:
// none
func (builder *LibP2PNodeBuilder) OverrideNodeConstructor(f p2p.NodeConstructor) p2p.NodeBuilder {
	builder.createNode = f
	return builder
}

// OverrideDefaultRpcInspectorFactory overrides the default rpc inspector factory for the GossipSub protocol.
// The purpose of override is to allow the node to provide a custom rpc inspector factory for sake of testing or experimentation.
// Note: it is not recommended to override the default rpc inspector factory in production unless you know what you are doing.
// Args:
// - factory: custom rpc inspector factory
// Returns:
// - NodeBuilder: the node builder
func (builder *LibP2PNodeBuilder) OverrideDefaultRpcInspectorFactory(factory p2p.GossipSubRpcInspectorFactoryFunc) p2p.NodeBuilder {
	builder.gossipSubBuilder.OverrideDefaultRpcInspectorFactory(factory)
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
		builder.logger.Info().Msgf("default libp2p resource manager is enabled with metrics: %v", builder.networkKey)
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
	builder.logger = builder.logger.With().Str("local_peer_id", p2plogging.PeerId(h.ID())).Logger()

	var peerManager p2p.PeerManager
	if builder.peerManagerConfig.UpdateInterval > 0 {
		connector, err := builder.peerManagerConfig.ConnectorFactory(h)
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2p connector: %w", err)
		}
		peerUpdater, err := connection.NewPeerUpdater(
			&connection.PeerUpdaterConfig{
				PruneConnections: builder.peerManagerConfig.ConnectionPruning,
				Logger:           builder.logger,
				Host:             connection.NewConnectorHost(h),
				Connector:        connector,
			})
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2p connector: %w", err)
		}

		peerManager = connection.NewPeerManager(builder.logger, builder.peerManagerConfig.UpdateInterval, peerUpdater)

		if builder.unicastConfig.RateLimiterDistributor != nil {
			builder.unicastConfig.RateLimiterDistributor.AddConsumer(peerManager)
		}
	}

	node, err := builder.createNode(&p2p.NodeConfig{
		Parameters: &p2p.NodeParameters{
			EnableProtectedStreams: builder.unicastConfig.EnableStreamProtection,
		},
		Logger:               builder.logger,
		Host:                 h,
		PeerManager:          peerManager,
		DisallowListCacheCfg: builder.disallowListCacheCfg,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create libp2p node: %w", err)
	}

	if builder.connGater != nil {
		builder.connGater.SetDisallowListOracle(node)
	}

	unicastManager, err := unicast.NewUnicastManager(&unicast.ManagerConfig{
		Logger:        builder.logger,
		StreamFactory: stream.NewLibP2PStreamFactory(h),
		SporkId:       builder.sporkId,
		Metrics:       builder.metricsConfig.Metrics,
		Parameters:    &builder.unicastConfig.UnicastManager,
		UnicastConfigCacheFactory: func(configFactory func() unicast.Config) unicast.ConfigCache {
			return unicastcache.NewUnicastConfigCache(builder.unicastConfig.UnicastManager.ConfigCacheSize, builder.logger,
				metrics.DialConfigCacheMetricFactory(builder.metricsConfig.HeroCacheFactory, builder.networkingType),
				configFactory)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not create unicast manager: %w", err)
	}
	node.SetUnicastManager(unicastManager)

	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			if builder.routingFactory != nil {
				routingSystem, err := builder.routingFactory(ctx, h)
				if err != nil {
					ctx.Throw(fmt.Errorf("could not create routing system: %w", err))
				}
				if err := node.SetRouting(routingSystem); err != nil {
					ctx.Throw(fmt.Errorf("could not set routing system: %w", err))
				}
				builder.gossipSubBuilder.SetRoutingSystem(routingSystem)
				builder.logger.Debug().Msg("routing system created")
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

// DefaultNodeBuilder returns a node builder.
func DefaultNodeBuilder(
	logger zerolog.Logger,
	address string,
	networkingType flownet.NetworkingType,
	flowKey fcrypto.PrivateKey,
	sporkId flow.Identifier,
	idProvider module.IdentityProvider,
	metricsCfg *p2pbuilderconfig.MetricsConfig,
	resolver madns.BasicResolver,
	role string,
	connGaterCfg *p2pbuilderconfig.ConnectionGaterConfig,
	peerManagerCfg *p2pbuilderconfig.PeerManagerConfig,
	gossipCfg *p2pconfig.GossipSubParameters,
	rCfg *p2pconfig.ResourceManagerConfig,
	uniCfg *p2pbuilderconfig.UnicastConfig,
	connMgrConfig *netconf.ConnectionManager,
	disallowListCacheCfg *p2p.DisallowListCacheConfig,
	dhtSystemActivation DhtSystemActivation,
) (p2p.NodeBuilder, error) {

	connManager, err := connection.NewConnManager(logger, metricsCfg.Metrics, connMgrConfig)
	if err != nil {
		return nil, fmt.Errorf("could not create connection manager: %w", err)
	}

	// set the default connection gater peer filters for both InterceptPeerDial and InterceptSecured callbacks
	peerFilter := notEjectedPeerFilter(idProvider)
	peerFilters := []p2p.PeerFilter{peerFilter}

	connGater := connection.NewConnGater(
		logger,
		idProvider,
		connection.WithOnInterceptPeerDialFilters(append(peerFilters, connGaterCfg.InterceptPeerDialFilters...)),
		connection.WithOnInterceptSecuredFilters(append(peerFilters, connGaterCfg.InterceptSecuredFilters...)))

	builder := NewNodeBuilder(logger,
		gossipCfg,
		metricsCfg,
		networkingType,
		address,
		flowKey,
		sporkId,
		idProvider,
		rCfg, peerManagerCfg,
		disallowListCacheCfg,
		uniCfg).
		SetBasicResolver(resolver).
		SetConnectionManager(connManager).
		SetConnectionGater(connGater)

	if role != "ghost" {
		r, err := flow.ParseRole(role)
		if err != nil {
			return nil, fmt.Errorf("could not parse role: %w", err)
		}
		builder.SetSubscriptionFilter(subscription.NewRoleBasedFilter(r, idProvider))

		if dhtSystemActivation == DhtSystemEnabled {
			builder.SetRoutingSystem(
				func(ctx context.Context, host host.Host) (routing.Routing, error) {
					return dht.NewDHT(ctx, host, protocols.FlowDHTProtocolID(sporkId), logger, metricsCfg.Metrics, dht.AsServer())
				})
		}
	}

	return builder, nil
}
