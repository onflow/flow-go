package scoring

import (
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	netcache "github.com/onflow/flow-go/network/p2p/cache"
	p2pconfig "github.com/onflow/flow-go/network/p2p/config"
	"github.com/onflow/flow-go/network/p2p/scoring/internal"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/utils/logging"
)

// ScoreOption is a functional option for configuring the peer scoring system.
// TODO: rename it to ScoreManager.
type ScoreOption struct {
	component.Component
	logger zerolog.Logger

	peerScoreParams         *pubsub.PeerScoreParams
	peerThresholdParams     *pubsub.PeerScoreThresholds
	defaultTopicScoreParams *pubsub.TopicScoreParams
	validator               p2p.SubscriptionValidator
	appScoreFunc            func(peer.ID) float64
}

type ScoreOptionConfig struct {
	logger                           zerolog.Logger
	params                           p2pconfig.ScoringParameters
	provider                         module.IdentityProvider
	heroCacheMetricsFactory          metrics.HeroCacheMetricsFactory
	appScoreFunc                     func(peer.ID) float64
	topicParams                      []func(map[string]*pubsub.TopicScoreParams)
	registerNotificationConsumerFunc func(p2p.GossipSubInvCtrlMsgNotifConsumer)
	getDuplicateMessageCount         func(id peer.ID) float64
	scoringRegistryMetricsCollector  module.GossipSubScoringRegistryMetrics
	networkingType                   network.NetworkingType
}

// NewScoreOptionConfig creates a new configuration for the GossipSub peer scoring option.
// Args:
// - logger: the logger to use.
// - hcMetricsFactory: HeroCache metrics factory to create metrics for the scoring-related caches.
// - idProvider: the identity provider to use.
// - networkingType: the networking type to use, public or private.
// Returns:
// - a new configuration for the GossipSub peer scoring option.
func NewScoreOptionConfig(logger zerolog.Logger,
	params p2pconfig.ScoringParameters,
	hcMetricsFactory metrics.HeroCacheMetricsFactory,
	scoringRegistryMetricsCollector module.GossipSubScoringRegistryMetrics,
	idProvider module.IdentityProvider,
	getDuplicateMessageCount func(id peer.ID) float64,
	networkingType network.NetworkingType) *ScoreOptionConfig {
	return &ScoreOptionConfig{
		logger:                          logger.With().Str("module", "pubsub_score_option").Logger(),
		provider:                        idProvider,
		params:                          params,
		heroCacheMetricsFactory:         hcMetricsFactory,
		topicParams:                     make([]func(map[string]*pubsub.TopicScoreParams), 0),
		networkingType:                  networkingType,
		getDuplicateMessageCount:        getDuplicateMessageCount,
		scoringRegistryMetricsCollector: scoringRegistryMetricsCollector,
	}
}

// OverrideAppSpecificScoreFunction sets the app specific penalty function for the penalty option.
// It is used to calculate the app specific penalty of a peer.
// If the app specific penalty function is not set, the default one is used.
// Note that it is always safer to use the default one, unless you know what you are doing.
// It is safe to call this method multiple times, the last call will be used.
func (c *ScoreOptionConfig) OverrideAppSpecificScoreFunction(appSpecificScoreFunction func(peer.ID) float64) {
	c.appScoreFunc = appSpecificScoreFunction
}

// OverrideTopicScoreParams overrides the topic score parameters for the given topic.
// It is used to override the default topic score parameters for a specific topic.
// If the topic score parameters are not set, the default ones will be used.
func (c *ScoreOptionConfig) OverrideTopicScoreParams(topic channels.Topic, topicScoreParams *pubsub.TopicScoreParams) {
	c.topicParams = append(c.topicParams, func(topics map[string]*pubsub.TopicScoreParams) {
		topics[topic.String()] = topicScoreParams
	})
}

// SetRegisterNotificationConsumerFunc sets the function to register the notification consumer for the penalty option.
// ScoreOption uses this function to register the notification consumer for the pubsub system so that it can receive
// notifications of invalid control messages.
func (c *ScoreOptionConfig) SetRegisterNotificationConsumerFunc(f func(p2p.GossipSubInvCtrlMsgNotifConsumer)) {
	c.registerNotificationConsumerFunc = f
}

// NewScoreOption creates a new penalty option with the given configuration.
func NewScoreOption(cfg *ScoreOptionConfig, provider p2p.SubscriptionProvider) (*ScoreOption, error) {
	throttledSampler := logging.BurstSampler(cfg.params.PeerScoring.Protocol.MaxDebugLogs, time.Second)
	logger := cfg.logger.With().
		Str("module", "pubsub_score_option").
		Logger().
		Sample(zerolog.LevelSampler{
			TraceSampler: throttledSampler,
			DebugSampler: throttledSampler,
		})

	validator := NewSubscriptionValidator(cfg.logger, provider)
	scoreRegistry, err := NewGossipSubAppSpecificScoreRegistry(&GossipSubAppSpecificScoreRegistryConfig{
		Logger:                  logger,
		Penalty:                 cfg.params.ScoringRegistryParameters.MisbehaviourPenalties,
		Validator:               validator,
		IdProvider:              cfg.provider,
		HeroCacheMetricsFactory: cfg.heroCacheMetricsFactory,
		AppScoreCacheFactory: func() p2p.GossipSubApplicationSpecificScoreCache {
			collector := metrics.NewGossipSubApplicationSpecificScoreCacheMetrics(cfg.heroCacheMetricsFactory, cfg.networkingType)
			return internal.NewAppSpecificScoreCache(cfg.params.ScoringRegistryParameters.SpamRecordCache.CacheSize, cfg.logger, collector)
		},
		SpamRecordCacheFactory: func() p2p.GossipSubSpamRecordCache {
			collector := metrics.GossipSubSpamRecordCacheMetricsFactory(cfg.heroCacheMetricsFactory, cfg.networkingType)
			return netcache.NewGossipSubSpamRecordCache(cfg.params.ScoringRegistryParameters.SpamRecordCache.CacheSize, cfg.logger, collector,
				InitAppScoreRecordStateFunc(cfg.params.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor),
				DefaultDecayFunction(cfg.params.ScoringRegistryParameters.SpamRecordCache.Decay))
		},
		GetDuplicateMessageCount: func(id peer.ID) float64 {
			return cfg.getDuplicateMessageCount(id)
		},
		Parameters:                cfg.params.ScoringRegistryParameters.AppSpecificScore,
		NetworkingType:            cfg.networkingType,
		AppSpecificScoreParams:    cfg.params.PeerScoring.Protocol.AppSpecificScore,
		DuplicateMessageThreshold: cfg.params.PeerScoring.Protocol.AppSpecificScore.DuplicateMessageThreshold,
		Collector:                 cfg.scoringRegistryMetricsCollector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gossipsub app specific score registry: %w", err)
	}

	s := &ScoreOption{
		logger:    logger,
		validator: validator,
		peerScoreParams: &pubsub.PeerScoreParams{
			Topics: make(map[string]*pubsub.TopicScoreParams),
			// we don't set all the parameters, so we skip the atomic validation.
			// atomic validation fails initialization if any parameter is not set.
			SkipAtomicValidation: cfg.params.PeerScoring.Internal.TopicParameters.SkipAtomicValidation,
			// DecayInterval is the interval over which we decay the effect of past behavior, so that
			// a good or bad behavior will not have a permanent effect on the penalty. It is also the interval
			// that GossipSub uses to refresh the scores of all peers.
			DecayInterval: cfg.params.PeerScoring.Internal.DecayInterval,
			// DecayToZero defines the maximum value below which a peer scoring counter is reset to zero.
			// This is to prevent the counter from decaying to a very small value.
			// When a counter hits the DecayToZero threshold, it means that the peer did not exhibit the behavior
			// for a long time, and we can reset the counter.
			DecayToZero: cfg.params.PeerScoring.Internal.DecayToZero,
			// AppSpecificWeight is the weight of the application specific penalty.
			AppSpecificWeight: cfg.params.PeerScoring.Internal.AppSpecificScoreWeight,
			// PenaltyThreshold is the threshold above which a peer is penalized for GossipSub-level misbehaviors.
			BehaviourPenaltyThreshold: cfg.params.PeerScoring.Internal.Behaviour.PenaltyThreshold,
			// PenaltyWeight is the weight of the GossipSub-level penalty.
			BehaviourPenaltyWeight: cfg.params.PeerScoring.Internal.Behaviour.PenaltyWeight,
			// PenaltyDecay is the decay of the GossipSub-level penalty (applied every decay interval).
			BehaviourPenaltyDecay: cfg.params.PeerScoring.Internal.Behaviour.PenaltyDecay,
		},
		peerThresholdParams: &pubsub.PeerScoreThresholds{
			GossipThreshold:             cfg.params.PeerScoring.Internal.Thresholds.Gossip,
			PublishThreshold:            cfg.params.PeerScoring.Internal.Thresholds.Publish,
			GraylistThreshold:           cfg.params.PeerScoring.Internal.Thresholds.Graylist,
			AcceptPXThreshold:           cfg.params.PeerScoring.Internal.Thresholds.AcceptPX,
			OpportunisticGraftThreshold: cfg.params.PeerScoring.Internal.Thresholds.OpportunisticGraft,
		},
		defaultTopicScoreParams: &pubsub.TopicScoreParams{
			TopicWeight:                     cfg.params.PeerScoring.Internal.TopicParameters.TopicWeight,
			SkipAtomicValidation:            cfg.params.PeerScoring.Internal.TopicParameters.SkipAtomicValidation,
			InvalidMessageDeliveriesWeight:  cfg.params.PeerScoring.Internal.TopicParameters.InvalidMessageDeliveriesWeight,
			InvalidMessageDeliveriesDecay:   cfg.params.PeerScoring.Internal.TopicParameters.InvalidMessageDeliveriesDecay,
			TimeInMeshQuantum:               cfg.params.PeerScoring.Internal.TopicParameters.TimeInMeshQuantum,
			MeshMessageDeliveriesWeight:     cfg.params.PeerScoring.Internal.TopicParameters.MeshDeliveriesWeight,
			MeshMessageDeliveriesDecay:      cfg.params.PeerScoring.Internal.TopicParameters.MeshMessageDeliveriesDecay,
			MeshMessageDeliveriesCap:        cfg.params.PeerScoring.Internal.TopicParameters.MeshMessageDeliveriesCap,
			MeshMessageDeliveriesThreshold:  cfg.params.PeerScoring.Internal.TopicParameters.MeshMessageDeliveryThreshold,
			MeshMessageDeliveriesWindow:     cfg.params.PeerScoring.Internal.TopicParameters.MeshMessageDeliveriesWindow,
			MeshMessageDeliveriesActivation: cfg.params.PeerScoring.Internal.TopicParameters.MeshMessageDeliveryActivation,
		},
		appScoreFunc: scoreRegistry.AppSpecificScoreFunc(),
	}

	// set the app specific penalty function for the penalty option
	// if the app specific penalty function is not set, use the default one
	if cfg.appScoreFunc != nil {
		s.appScoreFunc = cfg.appScoreFunc
		s.logger.
			Warn().
			Str(logging.KeyNetworkingSecurity, "true").
			Msg("app specific score function is overridden, should never happen in production")
	}

	if cfg.params.PeerScoring.Internal.DecayInterval > 0 && cfg.params.PeerScoring.Internal.DecayInterval != s.peerScoreParams.DecayInterval {
		// overrides the default decay interval if the decay interval is set.
		s.peerScoreParams.DecayInterval = cfg.params.PeerScoring.Internal.DecayInterval
		s.logger.
			Warn().
			Str(logging.KeyNetworkingSecurity, "true").
			Dur("decay_interval_ms", cfg.params.PeerScoring.Internal.DecayInterval).
			Msg("decay interval is overridden, should never happen in production")
	}

	// registers the score registry as the consumer of the invalid control message notifications
	if cfg.registerNotificationConsumerFunc != nil {
		cfg.registerNotificationConsumerFunc(scoreRegistry)
	}

	s.peerScoreParams.AppSpecificScore = s.appScoreFunc

	// apply the topic penalty parameters if any.
	for _, topicParams := range cfg.topicParams {
		topicParams(s.peerScoreParams.Topics)
	}

	s.Component = component.NewComponentManagerBuilder().AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		s.logger.Info().Msg("starting score registry")
		scoreRegistry.Start(ctx)
		select {
		case <-ctx.Done():
			s.logger.Warn().Msg("stopping score registry; context done")
		case <-scoreRegistry.Ready():
			s.logger.Info().Msg("score registry started")
			ready()
			s.logger.Info().Msg("score registry ready")
		}

		<-ctx.Done()
		s.logger.Info().Msg("stopping score registry")
		<-scoreRegistry.Done()
		s.logger.Info().Msg("score registry stopped")
	}).Build()

	return s, nil
}

func (s *ScoreOption) BuildFlowPubSubScoreOption() (*pubsub.PeerScoreParams, *pubsub.PeerScoreThresholds) {
	s.logger.Info().
		Float64("gossip_threshold", s.peerThresholdParams.GossipThreshold).
		Float64("publish_threshold", s.peerThresholdParams.PublishThreshold).
		Float64("graylist_threshold", s.peerThresholdParams.GraylistThreshold).
		Float64("accept_px_threshold", s.peerThresholdParams.AcceptPXThreshold).
		Float64("opportunistic_graft_threshold", s.peerThresholdParams.OpportunisticGraftThreshold).
		Msg("pubsub score thresholds are set")

	for topic, topicParams := range s.peerScoreParams.Topics {
		topicScoreParamLogger := utils.TopicScoreParamsLogger(s.logger, topic, topicParams)
		topicScoreParamLogger.Info().
			Msg("pubsub score topic parameters are set for topic")
	}

	return s.peerScoreParams, s.peerThresholdParams
}

// TopicScoreParams returns the topic score parameters for the given topic. If the topic
// score parameters are not set, it returns the default topic score parameters.
// The custom topic parameters are set at the initialization of the score option.
// Args:
// - topic: the topic for which the score parameters are requested.
// Returns:
//   - the topic score parameters for the given topic, or the default topic score parameters if
//     the topic score parameters are not set.
func (s *ScoreOption) TopicScoreParams(topic *pubsub.Topic) *pubsub.TopicScoreParams {
	params, exists := s.peerScoreParams.Topics[topic.String()]
	if !exists {
		return s.defaultTopicScoreParams
	}
	return params
}
