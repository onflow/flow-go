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

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	"github.com/onflow/flow-go/network/p2p/inspector"
	"github.com/onflow/flow-go/network/p2p/p2pnode"

	"github.com/onflow/flow-go/network/p2p/subscription"
	"github.com/onflow/flow-go/network/p2p/utils"

	"github.com/onflow/flow-go/network/p2p/dht"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/network/p2p/unicast"
)

const (
	defaultMemoryLimitRatio     = 0.2 // flow default
	defaultFileDescriptorsRatio = 0.5 // libp2p default
)

// LibP2PFactoryFunc is a factory function type for generating libp2p Node instances.
type LibP2PFactoryFunc func() (p2p.LibP2PNode, error)
type GossipSubFactoryFunc func(context.Context, zerolog.Logger, host.Host, p2p.PubSubAdapterConfig) (p2p.PubSubAdapter, error)
type CreateNodeFunc func(logger zerolog.Logger,
	host host.Host,
	pCache *p2pnode.ProtocolPeerCache,
	uniMgr *unicast.Manager,
	peerManager *connection.PeerManager) p2p.LibP2PNode
type GossipSubAdapterConfigFunc func(*p2p.BasePubSubAdapterConfig) p2p.PubSubAdapterConfig

// DefaultLibP2PNodeFactory returns a LibP2PFactoryFunc which generates the libp2p host initialized with the
// default options for the host, the pubsub and the ping service.
func DefaultLibP2PNodeFactory(log zerolog.Logger,
	address string,
	flowKey fcrypto.PrivateKey,
	sporkId flow.Identifier,
	idProvider module.IdentityProvider,
	metrics module.NetworkMetrics,
	resolver madns.BasicResolver,
	peerScoringEnabled bool,
	role string,
	onInterceptPeerDialFilters, onInterceptSecuredFilters []p2p.PeerFilter,
	connectionPruning bool,
	updateInterval time.Duration,
	rCfg *ResourceManagerConfig,
	rpcInspectorCfg *validation.ControlMsgValidationInspectorConfig,
	unicastRateLimiterDistributor p2p.UnicastRateLimiterDistributor,
) LibP2PFactoryFunc {
	return func() (p2p.LibP2PNode, error) {
		builder, err := DefaultNodeBuilder(log,
			address,
			flowKey,
			sporkId,
			idProvider,
			metrics,
			resolver,
			role,
			onInterceptPeerDialFilters,
			onInterceptSecuredFilters,
			peerScoringEnabled,
			connectionPruning,
			updateInterval,
			rCfg,
			rpcInspectorCfg,
			unicastRateLimiterDistributor)

		if err != nil {
			return nil, fmt.Errorf("could not create node builder: %w", err)
		}

		return builder.Build()
	}
}

type NodeBuilder interface {
	SetBasicResolver(madns.BasicResolver) NodeBuilder
	SetSubscriptionFilter(pubsub.SubscriptionFilter) NodeBuilder
	SetResourceManager(network.ResourceManager) NodeBuilder
	SetConnectionManager(connmgr.ConnManager) NodeBuilder
	SetConnectionGater(connmgr.ConnectionGater) NodeBuilder
	SetRoutingSystem(func(context.Context, host.Host) (routing.Routing, error)) NodeBuilder
	SetPeerManagerOptions(connectionPruning bool, updateInterval time.Duration) NodeBuilder
	EnableGossipSubPeerScoring(provider module.IdentityProvider, ops ...scoring.PeerScoreParamsOption) NodeBuilder
	SetCreateNode(CreateNodeFunc) NodeBuilder
	SetGossipSubFactory(GossipSubFactoryFunc, GossipSubAdapterConfigFunc) NodeBuilder
	SetRateLimiterDistributor(consumer p2p.UnicastRateLimiterDistributor) NodeBuilder
	SetRPCValidationInspectorConfig(cfg *validation.ControlMsgValidationInspectorConfig) NodeBuilder
	Build() (p2p.LibP2PNode, error)
}

// ResourceManagerConfig returns the resource manager configuration for the libp2p node.
// The resource manager is used to limit the number of open connections and streams (as well as any other resources
// used by libp2p) for each peer.
type ResourceManagerConfig struct {
	MemoryLimitRatio     float64 // maximum allowed fraction of memory to be allocated by the libp2p resources in (0,1]
	FileDescriptorsRatio float64 // maximum allowed fraction of file descriptors to be allocated by the libp2p resources in (0,1]
}

func DefaultResourceManagerConfig() *ResourceManagerConfig {
	return &ResourceManagerConfig{
		MemoryLimitRatio:     defaultMemoryLimitRatio,
		FileDescriptorsRatio: defaultFileDescriptorsRatio,
	}
}

// DefaultRPCValidationConfig returns default RPC control message inspector config.
func DefaultRPCValidationConfig() *validation.ControlMsgValidationInspectorConfig {
	graftCfg, _ := validation.NewCtrlMsgValidationConfig(validation.ControlMsgGraft, validation.CtrlMsgValidationLimits{
		validation.UpperThresholdMapKey:  validation.DefaultGraftUpperThreshold,
		validation.SafetyThresholdMapKey: validation.DefaultGraftSafetyThreshold,
		validation.RateLimitMapKey:       validation.DefaultGraftRateLimit,
	})
	pruneCfg, _ := validation.NewCtrlMsgValidationConfig(validation.ControlMsgPrune, validation.CtrlMsgValidationLimits{
		validation.UpperThresholdMapKey:  validation.DefaultPruneUpperThreshold,
		validation.SafetyThresholdMapKey: validation.DefaultPruneSafetyThreshold,
		validation.RateLimitMapKey:       validation.DefaultPruneRateLimit,
	})
	return &validation.ControlMsgValidationInspectorConfig{
		GraftValidationCfg: graftCfg,
		PruneValidationCfg: pruneCfg,
	}
}

type LibP2PNodeBuilder struct {
	sporkID                      flow.Identifier
	addr                         string
	networkKey                   fcrypto.PrivateKey
	logger                       zerolog.Logger
	metrics                      module.LibP2PMetrics
	basicResolver                madns.BasicResolver
	subscriptionFilter           pubsub.SubscriptionFilter
	resourceManager              network.ResourceManager
	resourceManagerCfg           *ResourceManagerConfig
	connManager                  connmgr.ConnManager
	connGater                    connmgr.ConnectionGater
	idProvider                   module.IdentityProvider
	gossipSubFactory             GossipSubFactoryFunc
	gossipSubConfigFunc          GossipSubAdapterConfigFunc
	gossipSubPeerScoring         bool // whether to enable gossipsub peer scoring
	routingFactory               func(context.Context, host.Host) (routing.Routing, error)
	peerManagerEnablePruning     bool
	peerManagerUpdateInterval    time.Duration
	peerScoringParameterOptions  []scoring.PeerScoreParamsOption
	createNode                   CreateNodeFunc
	rateLimiterDistributor       p2p.UnicastRateLimiterDistributor
	rpcValidationInspectorConfig *validation.ControlMsgValidationInspectorConfig
}

func NewNodeBuilder(logger zerolog.Logger,
	metrics module.LibP2PMetrics,
	addr string,
	networkKey fcrypto.PrivateKey,
	sporkID flow.Identifier,
	rCfg *ResourceManagerConfig) *LibP2PNodeBuilder {
	return &LibP2PNodeBuilder{
		logger:                       logger,
		sporkID:                      sporkID,
		addr:                         addr,
		networkKey:                   networkKey,
		createNode:                   DefaultCreateNodeFunc,
		gossipSubFactory:             defaultGossipSubFactory(),
		gossipSubConfigFunc:          defaultGossipSubAdapterConfig(),
		metrics:                      metrics,
		resourceManagerCfg:           rCfg,
		rpcValidationInspectorConfig: DefaultRPCValidationConfig(),
	}
}

func defaultGossipSubFactory() GossipSubFactoryFunc {
	return func(ctx context.Context, logger zerolog.Logger, h host.Host, cfg p2p.PubSubAdapterConfig) (p2p.PubSubAdapter, error) {
		return p2pnode.NewGossipSubAdapter(ctx, logger, h, cfg)
	}
}

func defaultGossipSubAdapterConfig() GossipSubAdapterConfigFunc {
	return func(cfg *p2p.BasePubSubAdapterConfig) p2p.PubSubAdapterConfig {
		return p2pnode.NewGossipSubAdapterConfig(cfg)
	}

}

// SetBasicResolver sets the DNS resolver for the node.
func (builder *LibP2PNodeBuilder) SetBasicResolver(br madns.BasicResolver) NodeBuilder {
	builder.basicResolver = br
	return builder
}

// SetSubscriptionFilter sets the pubsub subscription filter for the node.
func (builder *LibP2PNodeBuilder) SetSubscriptionFilter(filter pubsub.SubscriptionFilter) NodeBuilder {
	builder.subscriptionFilter = filter
	return builder
}

// SetResourceManager sets the resource manager for the node.
func (builder *LibP2PNodeBuilder) SetResourceManager(manager network.ResourceManager) NodeBuilder {
	builder.resourceManager = manager
	return builder
}

// SetConnectionManager sets the connection manager for the node.
func (builder *LibP2PNodeBuilder) SetConnectionManager(manager connmgr.ConnManager) NodeBuilder {
	builder.connManager = manager
	return builder
}

// SetConnectionGater sets the connection gater for the node.
func (builder *LibP2PNodeBuilder) SetConnectionGater(gater connmgr.ConnectionGater) NodeBuilder {
	builder.connGater = gater
	return builder
}

// SetRoutingSystem sets the routing factory function.
func (builder *LibP2PNodeBuilder) SetRoutingSystem(f func(context.Context, host.Host) (routing.Routing, error)) NodeBuilder {
	builder.routingFactory = f
	return builder
}

// EnableGossipSubPeerScoring sets builder.gossipSubPeerScoring to true.
func (builder *LibP2PNodeBuilder) EnableGossipSubPeerScoring(provider module.IdentityProvider, ops ...scoring.PeerScoreParamsOption) NodeBuilder {
	builder.gossipSubPeerScoring = true
	builder.idProvider = provider
	builder.peerScoringParameterOptions = ops
	return builder
}

// SetPeerManagerOptions sets the peer manager options.
func (builder *LibP2PNodeBuilder) SetPeerManagerOptions(connectionPruning bool, updateInterval time.Duration) NodeBuilder {
	builder.peerManagerEnablePruning = connectionPruning
	builder.peerManagerUpdateInterval = updateInterval
	return builder
}

func (builder *LibP2PNodeBuilder) SetCreateNode(f CreateNodeFunc) NodeBuilder {
	builder.createNode = f
	return builder
}

func (builder *LibP2PNodeBuilder) SetRateLimiterDistributor(distributor p2p.UnicastRateLimiterDistributor) NodeBuilder {
	builder.rateLimiterDistributor = distributor
	return builder
}

func (builder *LibP2PNodeBuilder) SetGossipSubFactory(gf GossipSubFactoryFunc, cf GossipSubAdapterConfigFunc) NodeBuilder {
	builder.gossipSubFactory = gf
	builder.gossipSubConfigFunc = cf
	return builder
}

func (builder *LibP2PNodeBuilder) SetRPCValidationInspectorConfig(cfg *validation.ControlMsgValidationInspectorConfig) NodeBuilder {
	builder.rpcValidationInspectorConfig = cfg
	return builder
}

// Build creates a new libp2p node using the configured options.
func (builder *LibP2PNodeBuilder) Build() (p2p.LibP2PNode, error) {
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

	h, err := DefaultLibP2PHost(builder.addr, builder.networkKey, opts...)

	if err != nil {
		return nil, err
	}

	pCache, err := p2pnode.NewProtocolPeerCache(builder.logger, h)
	if err != nil {
		return nil, err
	}

	unicastManager := unicast.NewUnicastManager(builder.logger, unicast.NewLibP2PStreamFactory(h), builder.sporkID)

	var peerManager *connection.PeerManager
	if builder.peerManagerUpdateInterval > 0 {
		connector, err := connection.NewLibp2pConnector(builder.logger, h, builder.peerManagerEnablePruning)
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2p connector: %w", err)
		}

		peerManager = connection.NewPeerManager(builder.logger, builder.peerManagerUpdateInterval, connector)

		if builder.rateLimiterDistributor != nil {
			builder.rateLimiterDistributor.AddConsumer(peerManager)
		}
	}

	node := builder.createNode(builder.logger, h, pCache, unicastManager, peerManager)

	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			rsys, err := builder.routingFactory(ctx, h)
			if err != nil {
				ctx.Throw(fmt.Errorf("could not create libp2p node routing: %w", err))
			}

			node.SetRouting(rsys)

			gossipSubConfigs := builder.gossipSubConfigFunc(&p2p.BasePubSubAdapterConfig{
				MaxMessageSize: p2pnode.DefaultMaxPubSubMsgSize,
			})
			gossipSubConfigs.WithMessageIdFunction(utils.MessageID)
			gossipSubConfigs.WithRoutingDiscovery(rsys)
			if builder.subscriptionFilter != nil {
				gossipSubConfigs.WithSubscriptionFilter(builder.subscriptionFilter)
			}

			var scoreOpt *scoring.ScoreOption
			if builder.gossipSubPeerScoring {
				scoreOpt = scoring.NewScoreOption(builder.logger, builder.idProvider, builder.peerScoringParameterOptions...)
				gossipSubConfigs.WithScoreOption(scoreOpt)
			}

			// create aggregate RPC inspector
			gossipSubRPCInspector := inspector.NewAggregateRPCInspector()

			// create gossip metrics inspector
			gossipSubMetrics := p2pnode.NewGossipSubControlMessageMetrics(builder.metrics, builder.logger)
			rpcMetricsInspector := inspector.NewControlMsgMetricsInspector(gossipSubMetrics)
			gossipSubRPCInspector.AddInspector(rpcMetricsInspector)

			// create and start gossip control message validation inspector
			//rpcControlMsgInspector := validation.NewControlMsgValidationInspector(builder.logger, builder.rpcValidationInspectorConfig)
			//rpcControlMsgInspector.Start(ctx)
			//gossipSubRPCInspector.AddInspector(rpcControlMsgInspector)

			// The app-specific rpc inspector is a hook into the pubsub that is invoked upon receiving any incoming RPC
			gossipSubConfigs.WithAppSpecificRpcInspector(gossipSubRPCInspector)

			// builds GossipSub with the given factory
			gossipSub, err := builder.gossipSubFactory(ctx, builder.logger, h, gossipSubConfigs)
			if err != nil {
				ctx.Throw(fmt.Errorf("could not create gossipsub: %w", err))
			}

			if scoreOpt != nil {
				scoreOpt.SetSubscriptionProvider(scoring.NewSubscriptionProvider(builder.logger, gossipSub))
			}
			node.SetPubSub(gossipSub)

			ready()
			<-ctx.Done()

			err = node.Stop()
			if err != nil {
				// ignore context cancellation errors
				if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					ctx.Throw(fmt.Errorf("could not stop libp2p node: %w", err))
				}
			}
		}).
		Build()

	node.SetComponentManager(cm)

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
	pCache *p2pnode.ProtocolPeerCache,
	uniMgr *unicast.Manager,
	peerManager *connection.PeerManager) p2p.LibP2PNode {
	return p2pnode.NewNode(logger, host, pCache, uniMgr, peerManager)
}

// DefaultNodeBuilder returns a node builder.
func DefaultNodeBuilder(log zerolog.Logger,
	address string,
	flowKey fcrypto.PrivateKey,
	sporkId flow.Identifier,
	idProvider module.IdentityProvider,
	metrics module.LibP2PMetrics,
	resolver madns.BasicResolver,
	role string,
	onInterceptPeerDialFilters, onInterceptSecuredFilters []p2p.PeerFilter,
	peerScoringEnabled bool,
	connectionPruning bool,
	updateInterval time.Duration,
	rCfg *ResourceManagerConfig,
	rpcInspectorCfg *validation.ControlMsgValidationInspectorConfig,
	unicastRateLimiterDistributor p2p.UnicastRateLimiterDistributor) (NodeBuilder, error) {

	connManager, err := connection.NewConnManager(log, metrics, connection.DefaultConnManagerConfig())
	if err != nil {
		return nil, fmt.Errorf("could not create connection manager: %w", err)
	}

	// set the default connection gater peer filters for both InterceptPeerDial and InterceptSecured callbacks
	peerFilter := notEjectedPeerFilter(idProvider)
	peerFilters := []p2p.PeerFilter{peerFilter}

	connGater := connection.NewConnGater(log,
		idProvider,
		connection.WithOnInterceptPeerDialFilters(append(peerFilters, onInterceptPeerDialFilters...)),
		connection.WithOnInterceptSecuredFilters(append(peerFilters, onInterceptSecuredFilters...)))

	builder := NewNodeBuilder(log, metrics, address, flowKey, sporkId, rCfg).
		SetBasicResolver(resolver).
		SetConnectionManager(connManager).
		SetConnectionGater(connGater).
		SetRoutingSystem(func(ctx context.Context, host host.Host) (routing.Routing, error) {
			return dht.NewDHT(ctx, host, unicast.FlowDHTProtocolID(sporkId), log, metrics, dht.AsServer())
		}).
		SetPeerManagerOptions(connectionPruning, updateInterval).
		SetCreateNode(DefaultCreateNodeFunc).
		SetRateLimiterDistributor(unicastRateLimiterDistributor).
		SetRPCValidationInspectorConfig(rpcInspectorCfg)

	if peerScoringEnabled {
		builder.EnableGossipSubPeerScoring(idProvider)
	}

	if role != "ghost" {
		r, _ := flow.ParseRole(role)
		builder.SetSubscriptionFilter(subscription.NewRoleBasedFilter(r, idProvider))
	}

	return builder, nil
}
