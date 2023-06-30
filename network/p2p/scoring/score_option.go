package scoring

import (
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	netcache "github.com/onflow/flow-go/network/p2p/cache"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	DefaultAppSpecificScoreWeight = 1
	MaxAppSpecificPenalty         = float64(-100)
	MinAppSpecificPenalty         = -1
	MaxAppSpecificReward          = float64(100)

	// DefaultStakedIdentityReward is the default reward for staking peers. It is applied to the peer's score when
	// the peer does not have any misbehavior record, e.g., invalid subscription, invalid message, etc.
	// The purpose is to reward the staking peers for their contribution to the network and prioritize them in neighbor selection.
	DefaultStakedIdentityReward = MaxAppSpecificReward

	// DefaultUnknownIdentityPenalty is the default penalty for unknown identity. It is applied to the peer's score when
	// the peer is not in the identity list.
	DefaultUnknownIdentityPenalty = MaxAppSpecificPenalty

	// DefaultInvalidSubscriptionPenalty is the default penalty for invalid subscription. It is applied to the peer's score when
	// the peer subscribes to a topic that it is not authorized to subscribe to.
	DefaultInvalidSubscriptionPenalty = MaxAppSpecificPenalty

	// DefaultGossipThreshold when a peer's penalty drops below this threshold,
	// no gossip is emitted towards that peer and gossip from that peer is ignored.
	//
	// Validation Constraint: GossipThreshold >= PublishThreshold && GossipThreshold < 0
	//
	// How we use it:
	// As current max penalty is -100, we set the threshold to -99 so that all gossips
	// to and from peers with penalty -100 are ignored.
	DefaultGossipThreshold = -99

	// DefaultPublishThreshold when a peer's penalty drops below this threshold,
	// self-published messages are not propagated towards this peer.
	//
	// Validation Constraint:
	// PublishThreshold >= GraylistThreshold && PublishThreshold <= GossipThreshold && PublishThreshold < 0.
	//
	// How we use it:
	// As current max penalty is -100, we set the threshold to -99 so that all penalized peers are deprived of
	// receiving any published messages.
	DefaultPublishThreshold = -99

	// DefaultGraylistThreshold when a peer's penalty drops below this threshold, the peer is graylisted, i.e.,
	// incoming RPCs from the peer are ignored.
	//
	// Validation Constraint:
	// GraylistThreshold =< PublishThreshold && GraylistThreshold =< GossipThreshold && GraylistThreshold < 0
	//
	// How we use it:
	// As current max penalty is -100, we set the threshold to -99 so that all penalized peers are graylisted.
	DefaultGraylistThreshold = -99

	// DefaultAcceptPXThreshold when a peer sends us PX information with a prune, we only accept it and connect to the supplied
	// peers if the originating peer's penalty exceeds this threshold.
	//
	// Validation Constraint: must be non-negative.
	//
	// How we use it:
	// As current max reward is 100, we set the threshold to 99 so that we only receive supplied peers from
	// well-behaved peers.
	DefaultAcceptPXThreshold = 99

	// DefaultOpportunisticGraftThreshold when the median peer penalty in the mesh drops below this value,
	// the peer may select more peers with penalty above the median to opportunistically graft on the mesh.
	//
	// Validation Constraint: must be non-negative.
	//
	// How we use it:
	// We set it to the MaxAppSpecificReward + 1 so that we only opportunistically graft peers that are not access nodes (i.e., with MinAppSpecificPenalty),
	// or penalized peers (i.e., with MaxAppSpecificPenalty).
	DefaultOpportunisticGraftThreshold = MaxAppSpecificReward + 1

	// MaxDebugLogs sets the max number of debug/trace log events per second. Logs emitted above
	// this threshold are dropped.
	MaxDebugLogs = 50

	// defaultScoreCacheSize is the default size of the cache used to store the app specific penalty of peers.
	defaultScoreCacheSize = 1000

	// defaultDecayInterval is the default decay interval for the overall score of a peer at the GossipSub scoring
	// system. We set it to 1 minute so that it is not too short so that a malicious node can recover from a penalty
	// and is not too long so that a well-behaved node can't recover from a penalty.
	defaultDecayInterval = 1 * time.Minute

	// defaultDecayToZero is the default decay to zero for the overall score of a peer at the GossipSub scoring system.
	// It defines the maximum value below which a peer scoring counter is reset to zero.
	// This is to prevent the counter from decaying to a very small value.
	// The default value is 0.01, which means that a counter will be reset to zero if it decays to 0.01.
	// When a counter hits the DecayToZero threshold, it means that the peer did not exhibit the behavior
	// for a long time, and we can reset the counter.
	defaultDecayToZero = 0.01

	// defaultTopicSkipAtomicValidation is the default value for the skip atomic validation flag for topics.
	// We set it to true, which means gossipsub parameter validation will not fail if we leave some of the
	// topic parameters at their default values, i.e., zero. This is because we are not setting all
	// topic parameters at the current implementation.
	defaultTopicSkipAtomicValidation = true

	// defaultTopicInvalidMessageDeliveriesWeight this value is applied to the square of the number of invalid message deliveries on a topic.
	// It is used to penalize peers that send invalid messages. By an invalid message, we mean a message that is not signed by the
	// publisher, or a message that is not signed by the peer that sent it. We set it to -1.0, which means that with around 14 invalid
	// message deliveries within a gossipsub heartbeat interval, the peer will be disconnected.
	// The supporting math is as follows:
	// - each staked (i.e., authorized) peer is rewarded by the fixed reward of 100 (i.e., DefaultStakedIdentityReward).
	// - x invalid message deliveries will result in a penalty of x^2 * DefaultTopicInvalidMessageDeliveriesWeight, i.e., -x^2.
	// - the peer will be disconnected when its penalty reaches -100 (i.e., MaxAppSpecificPenalty).
	// - so, the maximum number of invalid message deliveries that a peer can have before being disconnected is sqrt(200/DefaultTopicInvalidMessageDeliveriesWeight) ~ 14.
	defaultTopicInvalidMessageDeliveriesWeight = -1.0

	// defaultTopicInvalidMessageDeliveriesDecay decay factor used to decay the number of invalid message deliveries.
	// The total number of invalid message deliveries is multiplied by this factor at each heartbeat interval to
	// decay the number of invalid message deliveries, and prevent the peer from being disconnected if it stops
	// sending invalid messages. We set it to 0.99, which means that the number of invalid message deliveries will
	// decay by 1% at each heartbeat interval.
	// The decay heartbeats are defined by the heartbeat interval of the gossipsub scoring system, which is 1 Minute (defaultDecayInterval).
	defaultTopicInvalidMessageDeliveriesDecay = .99

	// defaultTopicTimeInMeshQuantum is the default time in mesh quantum for the GossipSub scoring system. It is used to gauge
	// a discrete time interval for the time in mesh counter. We set it to 1 hour, which means that every one complete hour a peer is
	// in a topic mesh, the time in mesh counter will be incremented by 1 and is counted towards the availability score of the peer in that topic mesh.
	// The reason of setting it to 1 hour is that we want to reward peers that are in a topic mesh for a long time, and we want to avoid rewarding peers that
	// are churners, i.e., peers that join and leave a topic mesh frequently.
	defaultTopicTimeInMesh = time.Hour

	// defaultTopicWeight is the default weight of a topic in the GossipSub scoring system. The overall score of a peer in a topic mesh is
	// multiplied by the weight of the topic when calculating the overall score of the peer.
	// We set it to 1.0, which means that the overall score of a peer in a topic mesh is not affected by the weight of the topic.
	defaultTopicWeight = 1.0
)

// ScoreOption is a functional option for configuring the peer scoring system.
type ScoreOption struct {
	logger zerolog.Logger

	peerScoreParams     *pubsub.PeerScoreParams
	peerThresholdParams *pubsub.PeerScoreThresholds
	validator           p2p.SubscriptionValidator
	appScoreFunc        func(peer.ID) float64
}

type ScoreOptionConfig struct {
	logger                           zerolog.Logger
	provider                         module.IdentityProvider
	cacheSize                        uint32
	cacheMetrics                     module.HeroCacheMetrics
	appScoreFunc                     func(peer.ID) float64
	topicParams                      []func(map[string]*pubsub.TopicScoreParams)
	registerNotificationConsumerFunc func(p2p.GossipSubInvCtrlMsgNotifConsumer)
}

func NewScoreOptionConfig(logger zerolog.Logger, idProvider module.IdentityProvider) *ScoreOptionConfig {
	return &ScoreOptionConfig{
		logger:       logger,
		provider:     idProvider,
		cacheSize:    defaultScoreCacheSize,
		cacheMetrics: metrics.NewNoopCollector(), // no metrics by default
		topicParams:  make([]func(map[string]*pubsub.TopicScoreParams), 0),
	}
}

// SetCacheSize sets the size of the cache used to store the app specific penalty of peers.
// If the cache size is not set, the default value will be used.
// It is safe to call this method multiple times, the last call will be used.
func (c *ScoreOptionConfig) SetCacheSize(size uint32) {
	c.cacheSize = size
}

// SetCacheMetrics sets the cache metrics collector for the penalty option.
// It is used to collect metrics for the app specific penalty cache. If the cache metrics collector is not set,
// a no-op collector will be used.
// It is safe to call this method multiple times, the last call will be used.
func (c *ScoreOptionConfig) SetCacheMetrics(metrics module.HeroCacheMetrics) {
	c.cacheMetrics = metrics
}

// SetAppSpecificScoreFunction sets the app specific penalty function for the penalty option.
// It is used to calculate the app specific penalty of a peer.
// If the app specific penalty function is not set, the default one is used.
// Note that it is always safer to use the default one, unless you know what you are doing.
// It is safe to call this method multiple times, the last call will be used.
func (c *ScoreOptionConfig) SetAppSpecificScoreFunction(appSpecificScoreFunction func(peer.ID) float64) {
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
func NewScoreOption(cfg *ScoreOptionConfig) *ScoreOption {
	throttledSampler := logging.BurstSampler(MaxDebugLogs, time.Second)
	logger := cfg.logger.With().
		Str("module", "pubsub_score_option").
		Logger().
		Sample(zerolog.LevelSampler{
			TraceSampler: throttledSampler,
			DebugSampler: throttledSampler,
		})
	validator := NewSubscriptionValidator()
	scoreRegistry := NewGossipSubAppSpecificScoreRegistry(&GossipSubAppSpecificScoreRegistryConfig{
		Logger:     logger,
		Penalty:    DefaultGossipSubCtrlMsgPenaltyValue(),
		Validator:  validator,
		Init:       InitAppScoreRecordState,
		IdProvider: cfg.provider,
		CacheFactory: func() p2p.GossipSubSpamRecordCache {
			return netcache.NewGossipSubSpamRecordCache(cfg.cacheSize, cfg.logger, cfg.cacheMetrics, DefaultDecayFunction())
		},
	})
	s := &ScoreOption{
		logger:          logger,
		validator:       validator,
		peerScoreParams: defaultPeerScoreParams(),
	}

	// set the app specific penalty function for the penalty option
	// if the app specific penalty function is not set, use the default one
	if cfg.appScoreFunc == nil {
		s.appScoreFunc = scoreRegistry.AppSpecificScoreFunc()
	} else {
		s.appScoreFunc = cfg.appScoreFunc
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
	return s
}

func (s *ScoreOption) SetSubscriptionProvider(provider *SubscriptionProvider) error {
	return s.validator.RegisterSubscriptionProvider(provider)
}

func (s *ScoreOption) BuildFlowPubSubScoreOption() (*pubsub.PeerScoreParams, *pubsub.PeerScoreThresholds) {
	s.preparePeerScoreThresholds()

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

func (s *ScoreOption) preparePeerScoreThresholds() {
	s.peerThresholdParams = &pubsub.PeerScoreThresholds{
		GossipThreshold:             DefaultGossipThreshold,
		PublishThreshold:            DefaultPublishThreshold,
		GraylistThreshold:           DefaultGraylistThreshold,
		AcceptPXThreshold:           DefaultAcceptPXThreshold,
		OpportunisticGraftThreshold: DefaultOpportunisticGraftThreshold,
	}
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
		return defaultTopicScoreParams()
	}
	return params
}

func defaultPeerScoreParams() *pubsub.PeerScoreParams {
	return &pubsub.PeerScoreParams{
		Topics: make(map[string]*pubsub.TopicScoreParams),
		// we don't set all the parameters, so we skip the atomic validation.
		// atomic validation fails initialization if any parameter is not set.
		SkipAtomicValidation: true,
		// DecayInterval is the interval over which we decay the effect of past behavior. So that
		// a good or bad behavior will not have a permanent effect on the penalty.
		DecayInterval: defaultDecayInterval,
		// DecayToZero defines the maximum value below which a peer scoring counter is reset to zero.
		// This is to prevent the counter from decaying to a very small value.
		// When a counter hits the DecayToZero threshold, it means that the peer did not exhibit the behavior
		// for a long time, and we can reset the counter.
		DecayToZero: defaultDecayToZero,
		// AppSpecificWeight is the weight of the application specific penalty.
		AppSpecificWeight: DefaultAppSpecificScoreWeight,
	}
}

// defaultTopicScoreParams returns the default score params for topics.
func defaultTopicScoreParams() *pubsub.TopicScoreParams {
	return &pubsub.TopicScoreParams{
		TopicWeight:                    defaultTopicWeight,
		SkipAtomicValidation:           defaultTopicSkipAtomicValidation,
		InvalidMessageDeliveriesWeight: defaultTopicInvalidMessageDeliveriesWeight,
		InvalidMessageDeliveriesDecay:  defaultTopicInvalidMessageDeliveriesDecay,
		TimeInMeshQuantum:              defaultTopicTimeInMesh,
	}
}
