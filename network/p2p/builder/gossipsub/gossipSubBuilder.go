package gossipsubbuilder

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	p2pbuilderconfig "github.com/onflow/flow-go/network/p2p/builder/config"
	p2pconfig "github.com/onflow/flow-go/network/p2p/config"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	p2pnode "github.com/onflow/flow-go/network/p2p/node"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/network/p2p/tracer"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/utils/logging"
)

// The Builder struct is used to configure and create a new GossipSub pubsub system.
type Builder struct {
	networkType         network.NetworkingType
	sporkId             flow.Identifier
	logger              zerolog.Logger
	metricsCfg          *p2pbuilderconfig.MetricsConfig
	h                   host.Host
	subscriptionFilter  pubsub.SubscriptionFilter
	gossipSubFactory    p2p.GossipSubFactoryFunc
	gossipSubConfigFunc p2p.GossipSubAdapterConfigFunc
	rpcInspectorFactory p2p.GossipSubRpcInspectorFactoryFunc
	// gossipSubTracer is a callback interface that is called by the gossipsub implementation upon
	// certain events. Currently, we use it to log and observe the local mesh of the node.
	gossipSubTracer   p2p.PubSubTracer
	scoreOptionConfig *scoring.ScoreOptionConfig
	idProvider        module.IdentityProvider
	routingSystem     routing.Routing
	gossipSubCfg      *p2pconfig.GossipSubParameters
}

var _ p2p.GossipSubBuilder = (*Builder)(nil)

// SetHost sets the host of the builder.
// If the host has already been set, a fatal error is logged.
func (g *Builder) SetHost(h host.Host) {
	if g.h != nil {
		g.logger.Fatal().Msg("host has already been set")
		return
	}
	g.h = h
}

func (g *Builder) OverrideDefaultRpcInspectorFactory(factoryFunc p2p.GossipSubRpcInspectorFactoryFunc) {
	g.logger.Warn().Msg("overriding default rpc inspector factory, not recommended for production")
	g.rpcInspectorFactory = factoryFunc
}

// SetSubscriptionFilter sets the subscription filter of the builder.
// If the subscription filter has already been set, a fatal error is logged.
func (g *Builder) SetSubscriptionFilter(subscriptionFilter pubsub.SubscriptionFilter) {
	if g.subscriptionFilter != nil {
		g.logger.Fatal().Msg("subscription filter has already been set")
	}
	g.subscriptionFilter = subscriptionFilter
}

// SetGossipSubFactory sets the gossipsub factory of the builder.
// We expect the node to initialize with a default gossipsub factory. Hence, this function overrides the default config.
func (g *Builder) SetGossipSubFactory(gossipSubFactory p2p.GossipSubFactoryFunc) {
	if g.gossipSubFactory != nil {
		g.logger.Warn().Msg("gossipsub factory has already been set, overriding the previous factory.")
	}
	g.gossipSubFactory = gossipSubFactory
}

// SetGossipSubConfigFunc sets the gossipsub config function of the builder.
// We expect the node to initialize with a default gossipsub config. Hence, this function overrides the default config.
func (g *Builder) SetGossipSubConfigFunc(gossipSubConfigFunc p2p.GossipSubAdapterConfigFunc) {
	if g.gossipSubConfigFunc != nil {
		g.logger.Warn().Msg("gossipsub config function has already been set, overriding the previous config function.")
	}
	g.gossipSubConfigFunc = gossipSubConfigFunc
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
func (g *Builder) EnableGossipSubScoringWithOverride(override *p2p.PeerScoringConfigOverride) {
	g.gossipSubCfg.PeerScoringEnabled = true // TODO: we should enable peer scoring by default.
	if override == nil {
		return
	}
	if override.AppSpecificScoreParams != nil {
		g.logger.Warn().
			Str(logging.KeyNetworkingSecurity, "true").
			Msg("overriding app specific score params for gossipsub")
		g.scoreOptionConfig.OverrideAppSpecificScoreFunction(override.AppSpecificScoreParams)
	}
	if override.TopicScoreParams != nil {
		for topic, params := range override.TopicScoreParams {
			topicLogger := utils.TopicScoreParamsLogger(g.logger, topic.String(), params)
			topicLogger.Warn().
				Str(logging.KeyNetworkingSecurity, "true").
				Msg("overriding topic score params for gossipsub")
			g.scoreOptionConfig.OverrideTopicScoreParams(topic, params)
		}
	}
}

// SetRoutingSystem sets the routing system of the builder.
// If the routing system has already been set, a fatal error is logged.
func (g *Builder) SetRoutingSystem(routingSystem routing.Routing) {
	if g.routingSystem != nil {
		g.logger.Fatal().Msg("routing system has already been set")
		return
	}
	g.routingSystem = routingSystem
}

// NewGossipSubBuilder returns a new gossipsub builder.
// Args:
// - logger: the logger of the node.
// - metricsCfg: the metrics config of the node.
// - networkType: the network type of the node.
// - sporkId: the spork id of the node.
// - idProvider: the identity provider of the node.
// - rpcInspectorConfig: the rpc inspector config of the node.
// - subscriptionProviderPrams: the subscription provider params of the node.
// - meshTracer: gossipsub mesh tracer.
// Returns:
// - a new gossipsub builder.
// Note: the builder is not thread-safe. It should only be used in the main thread.
func NewGossipSubBuilder(logger zerolog.Logger,
	metricsCfg *p2pbuilderconfig.MetricsConfig,
	gossipSubCfg *p2pconfig.GossipSubParameters,
	networkType network.NetworkingType,
	sporkId flow.Identifier,
	idProvider module.IdentityProvider) *Builder {
	lg := logger.With().
		Str("component", "gossipsub").
		Str("network-type", networkType.String()).
		Logger()

	meshTracerCfg := &tracer.GossipSubMeshTracerConfig{
		Logger:         lg,
		Metrics:        metricsCfg.Metrics,
		IDProvider:     idProvider,
		LoggerInterval: gossipSubCfg.RpcTracer.LocalMeshLogInterval,
		RpcSentTracker: tracer.RpcSentTrackerConfig{
			CacheSize:            gossipSubCfg.RpcTracer.RPCSentTrackerCacheSize,
			WorkerQueueCacheSize: gossipSubCfg.RpcTracer.RPCSentTrackerQueueCacheSize,
			WorkerQueueNumber:    gossipSubCfg.RpcTracer.RpcSentTrackerNumOfWorkers,
		},
		DuplicateMessageTrackerCacheConfig: gossipSubCfg.RpcTracer.DuplicateMessageTrackerConfig,
		HeroCacheMetricsFactory:            metricsCfg.HeroCacheFactory,
		NetworkingType:                     networkType,
	}
	meshTracer := tracer.NewGossipSubMeshTracer(meshTracerCfg)

	b := &Builder{
		logger:              lg,
		metricsCfg:          metricsCfg,
		sporkId:             sporkId,
		networkType:         networkType,
		idProvider:          idProvider,
		gossipSubFactory:    defaultGossipSubFactory(),
		gossipSubConfigFunc: defaultGossipSubAdapterConfig(),
		scoreOptionConfig: scoring.NewScoreOptionConfig(lg,
			gossipSubCfg.ScoringParameters,
			metricsCfg.HeroCacheFactory,
			metricsCfg.Metrics,
			idProvider,
			meshTracer.DuplicateMessageCount,
			networkType,
		),
		gossipSubTracer:     meshTracer,
		gossipSubCfg:        gossipSubCfg,
		rpcInspectorFactory: defaultRpcInspectorFactory(meshTracer),
	}

	return b
}

func defaultRpcInspectorFactory(tracer p2p.PubSubTracer) p2p.GossipSubRpcInspectorFactoryFunc {
	return func(logger zerolog.Logger,
		sporkId flow.Identifier,
		rpcInspectorConfig *p2pconfig.RpcInspectorParameters,
		inspectorMetrics module.GossipSubMetrics,
		heroCacheMetrics metrics.HeroCacheMetricsFactory,
		networkingType network.NetworkingType,
		idProvider module.IdentityProvider,
		topicProvider func() p2p.TopicProvider,
		notificationConsumer p2p.GossipSubInvCtrlMsgNotifConsumer) (p2p.GossipSubMsgValidationRpcInspector, error) {
		return validation.NewControlMsgValidationInspector(&validation.InspectorParams{
			Logger:                  logger.With().Str("component", "rpc-inspector").Logger(),
			SporkID:                 sporkId,
			Config:                  &rpcInspectorConfig.Validation,
			HeroCacheMetricsFactory: heroCacheMetrics,
			IdProvider:              idProvider,
			InspectorMetrics:        inspectorMetrics,
			RpcTracker:              tracer,
			NetworkingType:          networkingType,
			InvalidControlMessageNotificationConsumer: notificationConsumer,
			TopicOracle: topicProvider,
		})
	}
}

// defaultGossipSubFactory returns the default gossipsub factory function. It is used to create the default gossipsub factory.
// Note: always use the default gossipsub factory function to create the gossipsub factory (unless you know what you are doing).
func defaultGossipSubFactory() p2p.GossipSubFactoryFunc {
	return func(ctx context.Context, logger zerolog.Logger, h host.Host, cfg p2p.PubSubAdapterConfig, clusterChangeConsumer p2p.CollectionClusterChangesConsumer) (p2p.PubSubAdapter, error) {
		return p2pnode.NewGossipSubAdapter(ctx, logger, h, cfg, clusterChangeConsumer)
	}
}

// defaultGossipSubAdapterConfig returns the default gossipsub config function. It is used to create the default gossipsub config.
// Note: always use the default gossipsub config function to create the gossipsub config (unless you know what you are doing).
func defaultGossipSubAdapterConfig() p2p.GossipSubAdapterConfigFunc {
	return func(cfg *p2p.BasePubSubAdapterConfig) p2p.PubSubAdapterConfig {
		return p2pnode.NewGossipSubAdapterConfig(cfg)
	}
}

// Build creates a new GossipSub pubsub system.
// It returns the newly created GossipSub pubsub system and any errors encountered during its creation.
// Arguments:
// - ctx: the irrecoverable context of the node.
//
// Returns:
// - p2p.PubSubAdapter: a GossipSub pubsub system for the libp2p node.
// - p2p.PeerScoreTracer: a peer score tracer for the GossipSub pubsub system (if enabled, otherwise nil).
// - error: if an error occurs during the creation of the GossipSub pubsub system, it is returned. Otherwise, nil is returned.
// Note that on happy path, the returned error is nil. Any error returned is unexpected and should be handled as irrecoverable.
func (g *Builder) Build(ctx irrecoverable.SignalerContext) (p2p.PubSubAdapter, error) {
	// placeholder for the gossipsub pubsub system that will be created (so that it can be passed around even
	// before it is created).
	var gossipSub p2p.PubSubAdapter

	gossipSubConfigs := g.gossipSubConfigFunc(&p2p.BasePubSubAdapterConfig{
		MaxMessageSize: p2pnode.DefaultMaxPubSubMsgSize,
	})
	gossipSubConfigs.WithMessageIdFunction(utils.MessageID)

	if g.routingSystem != nil {
		gossipSubConfigs.WithRoutingDiscovery(g.routingSystem)
	}

	if g.subscriptionFilter != nil {
		gossipSubConfigs.WithSubscriptionFilter(g.subscriptionFilter)
	}

	var scoreOpt *scoring.ScoreOption
	var scoreTracer p2p.PeerScoreTracer
	// currently, peer scoring is not supported for public networks.
	if g.gossipSubCfg.PeerScoringEnabled && g.networkType != network.PublicNetwork {
		// wires the gossipsub score option to the subscription provider.
		subscriptionProvider, err := scoring.NewSubscriptionProvider(&scoring.SubscriptionProviderConfig{
			Logger: g.logger,
			TopicProviderOracle: func() p2p.TopicProvider {
				// gossipSub has not been created yet, hence instead of passing it directly, we pass a function that returns it.
				// the cardinal assumption is this function is only invoked when the subscription provider is started, which is
				// after the gossipsub is created.
				return gossipSub
			},
			IdProvider:              g.idProvider,
			Params:                  &g.gossipSubCfg.SubscriptionProvider,
			HeroCacheMetricsFactory: g.metricsCfg.HeroCacheFactory,
			NetworkingType:          g.networkType,
		})
		if err != nil {
			return nil, fmt.Errorf("could not create subscription provider: %w", err)
		}
		scoreOpt, err = scoring.NewScoreOption(g.scoreOptionConfig, subscriptionProvider)
		if err != nil {
			return nil, fmt.Errorf("could not create gossipsub score option: %w", err)
		}
		gossipSubConfigs.WithScoreOption(scoreOpt)

		if g.gossipSubCfg.RpcTracer.ScoreTracerInterval > 0 {
			scoreTracer = tracer.NewGossipSubScoreTracer(g.logger, g.idProvider, g.metricsCfg.Metrics, g.gossipSubCfg.RpcTracer.ScoreTracerInterval)
			gossipSubConfigs.WithScoreTracer(scoreTracer)
		}
	} else {
		g.logger.Warn().
			Str(logging.KeyNetworkingSecurity, "true").
			Msg("gossipsub peer scoring is disabled")
	}

	rpcValidationInspector, err := g.rpcInspectorFactory(
		g.logger,
		g.sporkId,
		&g.gossipSubCfg.RpcInspector,
		g.metricsCfg.Metrics,
		g.metricsCfg.HeroCacheFactory,
		g.networkType,
		g.idProvider,
		func() p2p.TopicProvider {
			return gossipSub
		},
		scoreOpt)
	if err != nil {
		return nil, fmt.Errorf("failed to create new rpc valiadation inspector: %w", err)
	}
	gossipSubConfigs.WithRpcInspector(rpcValidationInspector)

	if g.gossipSubTracer != nil {
		gossipSubConfigs.WithTracer(g.gossipSubTracer)
	}

	if g.h == nil {
		return nil, fmt.Errorf("could not create gossipsub: host is nil")
	}

	gossipSub, err = g.gossipSubFactory(ctx, g.logger, g.h, gossipSubConfigs, rpcValidationInspector)
	if err != nil {
		return nil, fmt.Errorf("could not create gossipsub: %w", err)
	}

	return gossipSub, nil
}
