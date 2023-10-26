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

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/network/p2p/tracer"
	"github.com/onflow/flow-go/network/p2p/utils"
)

// The Builder struct is used to configure and create a new GossipSub pubsub system.
type Builder struct {
	logger                       zerolog.Logger
	metrics                      module.GossipSubMetrics
	h                            host.Host
	subscriptionFilter           pubsub.SubscriptionFilter
	gossipSubFactory             p2p.GossipSubFactoryFunc
	gossipSubConfigFunc          p2p.GossipSubAdapterConfigFunc
	gossipSubPeerScoring         bool          // whether to enable gossipsub peer scoring
	gossipSubScoreTracerInterval time.Duration // the interval at which the gossipsub score tracer logs the peer scores.
	// gossipSubTracer is a callback interface that is called by the gossipsub implementation upon
	// certain events. Currently, we use it to log and observe the local mesh of the node.
	gossipSubTracer   p2p.PubSubTracer
	scoreOptionConfig *scoring.ScoreOptionConfig
	idProvider        module.IdentityProvider
	routingSystem     routing.Routing
	rpcInspectorSuite p2p.GossipSubInspectorSuite
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

// SetIDProvider sets the identity provider of the builder.
// If the identity provider has already been set, a fatal error is logged.
func (g *Builder) SetIDProvider(idProvider module.IdentityProvider) {
	if g.idProvider != nil {
		g.logger.Fatal().Msg("id provider has already been set")
		return
	}

	g.idProvider = idProvider
	g.scoreOptionConfig.SetProvider(idProvider)
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

func (g *Builder) SetTopicScoreParams(topic channels.Topic, topicScoreParams *pubsub.TopicScoreParams) {
	g.scoreOptionConfig.SetTopicScoreParams(topic, topicScoreParams)
}

func (g *Builder) SetAppSpecificScoreParams(f func(peer.ID) float64) {
	g.scoreOptionConfig.SetAppSpecificScoreFunction(f)
}

// SetGossipSubRPCInspectorSuite sets the gossipsub rpc inspector suite of the builder. It contains the
// inspector function that is injected into the gossipsub rpc layer, as well as the notification distributors that
// are used to notify the app specific scoring mechanism of misbehaving peers..
func (g *Builder) SetGossipSubRPCInspectorSuite(inspectorSuite p2p.GossipSubInspectorSuite) {
	g.rpcInspectorSuite = inspectorSuite
}

func NewGossipSubBuilder(logger zerolog.Logger, metrics module.GossipSubMetrics) *Builder {
	lg := logger.With().Str("component", "gossipsub").Logger()
	return &Builder{
		logger:              lg,
		metrics:             metrics,
		gossipSubFactory:    defaultGossipSubFactory(),
		gossipSubConfigFunc: defaultGossipSubAdapterConfig(),
		scoreOptionConfig:   scoring.NewScoreOptionConfig(lg),
	}
}

func defaultGossipSubFactory() p2p.GossipSubFactoryFunc {
	return func(ctx context.Context, logger zerolog.Logger, h host.Host, cfg p2p.PubSubAdapterConfig) (p2p.PubSubAdapter, error) {
		return p2pnode.NewGossipSubAdapter(ctx, logger, h, cfg)
	}
}

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
func (g *Builder) Build(ctx irrecoverable.SignalerContext) (p2p.PubSubAdapter, p2p.PeerScoreTracer, error) {
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

	if g.rpcInspectorSuite != nil {
		gossipSubConfigs.WithInspectorSuite(g.rpcInspectorSuite)
	}

	var scoreOpt *scoring.ScoreOption
	var scoreTracer p2p.PeerScoreTracer
	if g.gossipSubPeerScoring {
		if g.rpcInspectorSuite != nil {
			g.scoreOptionConfig.SetRegisterNotificationConsumerFunc(g.rpcInspectorSuite.AddInvCtrlMsgNotifConsumer)
		}

		scoreOpt = scoring.NewScoreOption(g.scoreOptionConfig)
		gossipSubConfigs.WithScoreOption(scoreOpt)

		if g.gossipSubScoreTracerInterval > 0 {
			scoreTracer = tracer.NewGossipSubScoreTracer(
				g.logger,
				g.idProvider,
				g.metrics,
				g.gossipSubScoreTracerInterval)
			gossipSubConfigs.WithScoreTracer(scoreTracer)
		}

	}

	if g.gossipSubTracer != nil {
		gossipSubConfigs.WithTracer(g.gossipSubTracer)
	}

	if g.h == nil {
		return nil, nil, fmt.Errorf("could not create gossipsub: host is nil")
	}

	gossipSub, err := g.gossipSubFactory(ctx, g.logger, g.h, gossipSubConfigs)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create gossipsub: %w", err)
	}

	if scoreOpt != nil {
		err := scoreOpt.SetSubscriptionProvider(scoring.NewSubscriptionProvider(g.logger, gossipSub))
		if err != nil {
			return nil, nil, fmt.Errorf("could not set subscription provider: %w", err)
		}
	}

	return gossipSub, scoreTracer, nil
}
