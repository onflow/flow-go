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
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/distributor"
	"github.com/onflow/flow-go/network/p2p/inspector"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	p2pconfig "github.com/onflow/flow-go/network/p2p/p2pbuilder/config"
	inspectorbuilder "github.com/onflow/flow-go/network/p2p/p2pbuilder/inspector"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
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
	metricsCfg          *p2pconfig.MetricsConfig
	h                   host.Host
	subscriptionFilter  pubsub.SubscriptionFilter
	gossipSubFactory    p2p.GossipSubFactoryFunc
	gossipSubConfigFunc p2p.GossipSubAdapterConfigFunc
	// gossipSubTracer is a callback interface that is called by the gossipsub implementation upon
	// certain events. Currently, we use it to log and observe the local mesh of the node.
	gossipSubTracer          p2p.PubSubTracer
	scoreOptionConfig        *scoring.ScoreOptionConfig
	idProvider               module.IdentityProvider
	routingSystem            routing.Routing
	rpcInspectorSuiteFactory p2p.GossipSubRpcInspectorSuiteFactoryFunc
	gossipSubCfg             *p2pconf.GossipSubParameters
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

// OverrideDefaultRpcInspectorSuiteFactory overrides the default rpc inspector suite factory.
// Note: this function should only be used for testing purposes. Never override the default rpc inspector suite factory unless you know what you are doing.
func (g *Builder) OverrideDefaultRpcInspectorSuiteFactory(factory p2p.GossipSubRpcInspectorSuiteFactoryFunc) {
	g.logger.Warn().Msg("overriding default rpc inspector suite factory")
	g.rpcInspectorSuiteFactory = factory
}

// NewGossipSubBuilder returns a new gossipsub builder.
// Args:
// - logger: the logger of the node.
// - metricsCfg: the metrics config of the node.
// - networkType: the network type of the node.
// - sporkId: the spork id of the node.
// - idProvider: the identity provider of the node.
// - rpcInspectorConfig: the rpc inspector config of the node.
// Returns:
// - a new gossipsub builder.
// Note: the builder is not thread-safe. It should only be used in the main thread.
func NewGossipSubBuilder(logger zerolog.Logger,
	metricsCfg *p2pconfig.MetricsConfig,
	gossipSubCfg *p2pconf.GossipSubParameters,
	networkType network.NetworkingType,
	sporkId flow.Identifier,
	idProvider module.IdentityProvider) *Builder {
	lg := logger.With().
		Str("component", "gossipsub").
		Str("network-type", networkType.String()).
		Logger()

	meshTracerCfg := &tracer.GossipSubMeshTracerConfig{
		Logger:                             lg,
		Metrics:                            metricsCfg.Metrics,
		IDProvider:                         idProvider,
		LoggerInterval:                     gossipSubCfg.RpcTracer.LocalMeshLogInterval,
		RpcSentTrackerCacheSize:            gossipSubCfg.RpcTracer.RPCSentTrackerCacheSize,
		RpcSentTrackerWorkerQueueCacheSize: gossipSubCfg.RpcTracer.RPCSentTrackerQueueCacheSize,
		RpcSentTrackerNumOfWorkers:         gossipSubCfg.RpcTracer.RpcSentTrackerNumOfWorkers,
		HeroCacheMetricsFactory:            metricsCfg.HeroCacheFactory,
		NetworkingType:                     networkType,
	}
	meshTracer := tracer.NewGossipSubMeshTracer(meshTracerCfg)

	b := &Builder{
		logger:                   lg,
		metricsCfg:               metricsCfg,
		sporkId:                  sporkId,
		networkType:              networkType,
		idProvider:               idProvider,
		gossipSubFactory:         defaultGossipSubFactory(),
		gossipSubConfigFunc:      defaultGossipSubAdapterConfig(),
		scoreOptionConfig:        scoring.NewScoreOptionConfig(lg, gossipSubCfg.ScoringParameters, metricsCfg.HeroCacheFactory, idProvider, networkType),
		rpcInspectorSuiteFactory: defaultInspectorSuite(meshTracer),
		gossipSubTracer:          meshTracer,
		gossipSubCfg:             gossipSubCfg,
	}

	return b
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

// defaultInspectorSuite returns the default inspector suite factory function. It is used to create the default inspector suite.
// Inspector suite is utilized to inspect the incoming gossipsub rpc messages from different perspectives.
// Note: always use the default inspector suite factory function to create the inspector suite (unless you know what you are doing).
func defaultInspectorSuite(rpcTracker p2p.RpcControlTracking) p2p.GossipSubRpcInspectorSuiteFactoryFunc {
	return func(ctx irrecoverable.SignalerContext,
		logger zerolog.Logger,
		sporkId flow.Identifier,
		inspectorCfg *p2pconf.RpcInspectorParameters,
		gossipSubMetrics module.GossipSubMetrics,
		heroCacheMetricsFactory metrics.HeroCacheMetricsFactory,
		networkType network.NetworkingType,
		idProvider module.IdentityProvider,
		topicProvider func() p2p.TopicProvider) (p2p.GossipSubInspectorSuite, error) {
		metricsInspector := inspector.NewControlMsgMetricsInspector(logger,
			p2pnode.NewGossipSubControlMessageMetrics(gossipSubMetrics, logger),
			inspectorCfg.Metrics.NumberOfWorkers,
			[]queue.HeroStoreConfigOption{
				queue.WithHeroStoreSizeLimit(inspectorCfg.Metrics.CacheSize),
				queue.WithHeroStoreCollector(metrics.GossipSubRPCMetricsObserverInspectorQueueMetricFactory(heroCacheMetricsFactory, networkType)),
			}...)
		notificationDistributor := distributor.DefaultGossipSubInspectorNotificationDistributor(logger, []queue.HeroStoreConfigOption{
			queue.WithHeroStoreSizeLimit(inspectorCfg.NotificationCacheSize),
			queue.WithHeroStoreCollector(metrics.RpcInspectorNotificationQueueMetricFactory(heroCacheMetricsFactory, networkType))}...)

		params := &validation.InspectorParams{
			Logger:                  logger,
			SporkID:                 sporkId,
			Config:                  &inspectorCfg.Validation,
			Distributor:             notificationDistributor,
			HeroCacheMetricsFactory: heroCacheMetricsFactory,
			IdProvider:              idProvider,
			InspectorMetrics:        gossipSubMetrics,
			RpcTracker:              rpcTracker,
			NetworkingType:          networkType,
			TopicOracle:             topicProvider,
		}
		rpcValidationInspector, err := validation.NewControlMsgValidationInspector(params)
		if err != nil {
			return nil, fmt.Errorf("failed to create new control message valiadation inspector: %w", err)
		}
		return inspectorbuilder.NewGossipSubInspectorSuite(metricsInspector, rpcValidationInspector, notificationDistributor), nil
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

	inspectorSuite, err := g.rpcInspectorSuiteFactory(ctx,
		g.logger,
		g.sporkId,
		&g.gossipSubCfg.RpcInspector,
		g.metricsCfg.Metrics,
		g.metricsCfg.HeroCacheFactory,
		g.networkType,
		g.idProvider,
		func() p2p.TopicProvider {
			return gossipSub
		})
	if err != nil {
		return nil, fmt.Errorf("could not create gossipsub inspector suite: %w", err)
	}
	gossipSubConfigs.WithInspectorSuite(inspectorSuite)

	var scoreOpt *scoring.ScoreOption
	var scoreTracer p2p.PeerScoreTracer
	if g.gossipSubCfg.PeerScoringEnabled {
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

		g.scoreOptionConfig.SetRegisterNotificationConsumerFunc(inspectorSuite.AddInvalidControlMessageConsumer)
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

	if g.gossipSubTracer != nil {
		gossipSubConfigs.WithTracer(g.gossipSubTracer)
	}

	if g.h == nil {
		return nil, fmt.Errorf("could not create gossipsub: host is nil")
	}

	gossipSub, err = g.gossipSubFactory(ctx, g.logger, g.h, gossipSubConfigs, inspectorSuite)
	if err != nil {
		return nil, fmt.Errorf("could not create gossipsub: %w", err)
	}

	return gossipSub, nil
}
