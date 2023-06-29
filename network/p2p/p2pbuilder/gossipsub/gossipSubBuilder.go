package gossipsubbuilder

import (
	"context"
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
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
)

// The Builder struct is used to configure and create a new GossipSub pubsub system.
type Builder struct {
	networkType                  network.NetworkingType
	sporkId                      flow.Identifier
	logger                       zerolog.Logger
	metricsCfg                   *p2pconfig.MetricsConfig
	h                            host.Host
	subscriptionFilter           pubsub.SubscriptionFilter
	gossipSubFactory             p2p.GossipSubFactoryFunc
	gossipSubConfigFunc          p2p.GossipSubAdapterConfigFunc
	gossipSubPeerScoring         bool          // whether to enable gossipsub peer scoring
	gossipSubScoreTracerInterval time.Duration // the interval at which the gossipsub score tracer logs the peer scores.
	// gossipSubTracer is a callback interface that is called by the gossipsub implementation upon
	// certain events. Currently, we use it to log and observe the local mesh of the node.
	gossipSubTracer          p2p.PubSubTracer
	scoreOptionConfig        *scoring.ScoreOptionConfig
	idProvider               module.IdentityProvider
	routingSystem            routing.Routing
	rpcInspectorConfig       *p2pconf.GossipSubRPCInspectorsConfig
	rpcInspectorSuiteFactory p2p.GossipSubRpcInspectorSuiteFactoryFunc
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

// SetGossipSubPeerScoring sets the gossipsub peer scoring of the builder.
// If the gossipsub peer scoring flag has already been set, a fatal error is logged.
func (g *Builder) SetGossipSubPeerScoring(gossipSubPeerScoring bool) {
	if g.gossipSubPeerScoring {
		g.logger.Fatal().Msg("gossipsub peer scoring has already been set")
		return
	}
	g.gossipSubPeerScoring = gossipSubPeerScoring
}

// SetGossipSubScoreTracerInterval sets the gossipsub score tracer interval of the builder.
// If the gossipsub score tracer interval has already been set, a fatal error is logged.
func (g *Builder) SetGossipSubScoreTracerInterval(gossipSubScoreTracerInterval time.Duration) {
	if g.gossipSubScoreTracerInterval != time.Duration(0) {
		g.logger.Fatal().Msg("gossipsub score tracer interval has already been set")
		return
	}
	g.gossipSubScoreTracerInterval = gossipSubScoreTracerInterval
}

// SetGossipSubTracer sets the gossipsub tracer of the builder.
// If the gossipsub tracer has already been set, a fatal error is logged.
func (g *Builder) SetGossipSubTracer(gossipSubTracer p2p.PubSubTracer) {
	if g.gossipSubTracer != nil {
		g.logger.Fatal().Msg("gossipsub tracer has already been set")
		return
	}
	g.gossipSubTracer = gossipSubTracer
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

// SetTopicScoreParams sets the topic score params of the builder.
// There is a default topic score parameters that is used if this function is not called for a topic.
// However, if this function is called multiple times for a topic, the last topic score params will be used.
// Note: calling this function will override the default topic score params for the topic. Don't call this function
// unless you know what you are doing.
func (g *Builder) SetTopicScoreParams(topic channels.Topic, topicScoreParams *pubsub.TopicScoreParams) {
	g.scoreOptionConfig.OverrideTopicScoreParams(topic, topicScoreParams)
}

// SetAppSpecificScoreParams sets the app specific score params of the builder.
// There is no default app specific score function. However, if this function is called multiple times, the last function will be used.
func (g *Builder) SetAppSpecificScoreParams(f func(peer.ID) float64) {
	g.scoreOptionConfig.SetAppSpecificScoreFunction(f)
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
func NewGossipSubBuilder(
	logger zerolog.Logger,
	metricsCfg *p2pconfig.MetricsConfig,
	networkType network.NetworkingType,
	sporkId flow.Identifier,
	idProvider module.IdentityProvider,
	rpcInspectorConfig *p2pconf.GossipSubRPCInspectorsConfig,
) *Builder {
	lg := logger.With().Str("component", "gossipsub").Logger()
	b := &Builder{
		logger:                   lg,
		metricsCfg:               metricsCfg,
		sporkId:                  sporkId,
		networkType:              networkType,
		idProvider:               idProvider,
		gossipSubFactory:         defaultGossipSubFactory(),
		gossipSubConfigFunc:      defaultGossipSubAdapterConfig(),
		scoreOptionConfig:        scoring.NewScoreOptionConfig(lg, idProvider),
		rpcInspectorConfig:       rpcInspectorConfig,
		rpcInspectorSuiteFactory: defaultInspectorSuite(),
	}
	return b
}

// defaultGossipSubFactory returns the default gossipsub factory function. It is used to create the default gossipsub factory.
// Note: always use the default gossipsub factory function to create the gossipsub factory (unless you know what you are doing).
func defaultGossipSubFactory() p2p.GossipSubFactoryFunc {
	return func(
		ctx context.Context,
		logger zerolog.Logger,
		h host.Host,
		cfg p2p.PubSubAdapterConfig,
		clusterChangeConsumer p2p.CollectionClusterChangesConsumer) (p2p.PubSubAdapter, error) {
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
func defaultInspectorSuite() p2p.GossipSubRpcInspectorSuiteFactoryFunc {
	return func(
		logger zerolog.Logger,
		sporkId flow.Identifier,
		inspectorCfg *p2pconf.GossipSubRPCInspectorsConfig,
		gossipSubMetrics module.GossipSubMetrics,
		heroCacheMetricsFactory metrics.HeroCacheMetricsFactory,
		networkType network.NetworkingType,
		idProvider module.IdentityProvider) (p2p.GossipSubInspectorSuite, error) {
		metricsInspector := inspector.NewControlMsgMetricsInspector(
			logger,
			p2pnode.NewGossipSubControlMessageMetrics(gossipSubMetrics, logger),
			inspectorCfg.GossipSubRPCMetricsInspectorConfigs.NumberOfWorkers,
			[]queue.HeroStoreConfigOption{
				queue.WithHeroStoreSizeLimit(inspectorCfg.GossipSubRPCMetricsInspectorConfigs.CacheSize),
				queue.WithHeroStoreCollector(metrics.GossipSubRPCMetricsObserverInspectorQueueMetricFactory(heroCacheMetricsFactory, networkType)),
			}...)
		notificationDistributor := distributor.DefaultGossipSubInspectorNotificationDistributor(
			logger,
			[]queue.HeroStoreConfigOption{
				queue.WithHeroStoreSizeLimit(inspectorCfg.GossipSubRPCInspectorNotificationCacheSize),
				queue.WithHeroStoreCollector(metrics.RpcInspectorNotificationQueueMetricFactory(heroCacheMetricsFactory, networkType))}...)

		inspectMsgQueueCacheCollector := metrics.GossipSubRPCInspectorQueueMetricFactory(heroCacheMetricsFactory, networkType)
		clusterPrefixedCacheCollector := metrics.GossipSubRPCInspectorClusterPrefixedCacheMetricFactory(heroCacheMetricsFactory, networkType)
		rpcValidationInspector, err := validation.NewControlMsgValidationInspector(
			logger,
			sporkId,
			&inspectorCfg.GossipSubRPCValidationInspectorConfigs,
			notificationDistributor,
			inspectMsgQueueCacheCollector,
			clusterPrefixedCacheCollector,
			idProvider,
			gossipSubMetrics,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create new control message valiadation inspector: %w", err)
		}

		return inspectorbuilder.NewGossipSubInspectorSuite([]p2p.GossipSubRPCInspector{metricsInspector, rpcValidationInspector}, notificationDistributor), nil
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
	gossipSubConfigs := g.gossipSubConfigFunc(&p2p.BasePubSubAdapterConfig{
		MaxMessageSize: p2pnode.DefaultMaxPubSubMsgSize,
	})
	gossipSubConfigs.WithMessageIdFunction(utils.MessageID)

	if g.routingSystem == nil {
		return nil, fmt.Errorf("could not create gossipsub: routing system is nil")
	}
	gossipSubConfigs.WithRoutingDiscovery(g.routingSystem)

	if g.subscriptionFilter != nil {
		gossipSubConfigs.WithSubscriptionFilter(g.subscriptionFilter)
	}

	inspectorSuite, err := g.rpcInspectorSuiteFactory(
		g.logger,
		g.sporkId,
		g.rpcInspectorConfig,
		g.metricsCfg.Metrics,
		g.metricsCfg.HeroCacheFactory,
		g.networkType,
		g.idProvider)
	if err != nil {
		return nil, fmt.Errorf("could not create gossipsub inspector suite: %w", err)
	}
	gossipSubConfigs.WithInspectorSuite(inspectorSuite)

	var scoreOpt *scoring.ScoreOption
	var scoreTracer p2p.PeerScoreTracer
	if g.gossipSubPeerScoring {
		g.scoreOptionConfig.SetRegisterNotificationConsumerFunc(inspectorSuite.AddInvalidControlMessageConsumer)
		scoreOpt = scoring.NewScoreOption(g.scoreOptionConfig)
		gossipSubConfigs.WithScoreOption(scoreOpt)

		if g.gossipSubScoreTracerInterval > 0 {
			scoreTracer = tracer.NewGossipSubScoreTracer(
				g.logger,
				g.idProvider,
				g.metricsCfg.Metrics,
				g.gossipSubScoreTracerInterval)
			gossipSubConfigs.WithScoreTracer(scoreTracer)
		}

	}

	if g.gossipSubTracer != nil {
		gossipSubConfigs.WithTracer(g.gossipSubTracer)
	}

	if g.h == nil {
		return nil, fmt.Errorf("could not create gossipsub: host is nil")
	}

	gossipSub, err := g.gossipSubFactory(ctx, g.logger, g.h, gossipSubConfigs, inspectorSuite)
	if err != nil {
		return nil, fmt.Errorf("could not create gossipsub: %w", err)
	}

	if scoreOpt != nil {
		err := scoreOpt.SetSubscriptionProvider(scoring.NewSubscriptionProvider(g.logger, gossipSub))
		if err != nil {
			return nil, fmt.Errorf("could not set subscription provider: %w", err)
		}
	}

	return gossipSub, nil
}
