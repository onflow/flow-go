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
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	netcache "github.com/onflow/flow-go/network/p2p/cache"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
	"github.com/onflow/flow-go/network/p2p/scoring/internal"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// DefaultAppSpecificScoreWeight is the default weight for app-specific scores. It is used to scale the app-specific
	// scores to the same range as the other scores. At the current version, we don't distinguish between the app-specific
	// scores and the other scores, so we set it to 1.
	DefaultAppSpecificScoreWeight = 1

	// MaxAppSpecificReward is the default reward for well-behaving staked peers. If a peer does not have
	// any misbehavior record, e.g., invalid subscription, invalid message, etc., it will be rewarded with this score.
	MaxAppSpecificReward = float64(100)

	// MaxAppSpecificPenalty is the maximum penalty for sever offenses that we apply to a remote node score. The score
	// mechanism of GossipSub in Flow is designed in a way that all other infractions are penalized with a fraction of
	// this value. We have also set the other parameters such as DefaultGraylistThreshold, DefaultGossipThreshold and DefaultPublishThreshold to
	// be a bit higher than this, i.e., MaxAppSpecificPenalty + 1. This ensures that a node with a score of MaxAppSpecificPenalty
	// will be graylisted (i.e., all incoming and outgoing RPCs are rejected) and will not be able to publish or gossip any messages.
	MaxAppSpecificPenalty = -1 * MaxAppSpecificReward
	MinAppSpecificPenalty = -1

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
	DefaultGossipThreshold = MaxAppSpecificPenalty + 1

	// DefaultPublishThreshold when a peer's penalty drops below this threshold,
	// self-published messages are not propagated towards this peer.
	//
	// Validation Constraint:
	// PublishThreshold >= GraylistThreshold && PublishThreshold <= GossipThreshold && PublishThreshold < 0.
	//
	// How we use it:
	// As current max penalty is -100, we set the threshold to -99 so that all penalized peers are deprived of
	// receiving any published messages.
	DefaultPublishThreshold = MaxAppSpecificPenalty + 1

	// DefaultGraylistThreshold when a peer's penalty drops below this threshold, the peer is graylisted, i.e.,
	// incoming RPCs from the peer are ignored.
	//
	// Validation Constraint:
	// GraylistThreshold =< PublishThreshold && GraylistThreshold =< GossipThreshold && GraylistThreshold < 0
	//
	// How we use it:
	// As current max penalty is -100, we set the threshold to -99 so that all penalized peers are graylisted.
	DefaultGraylistThreshold = MaxAppSpecificPenalty + 1

	// DefaultAcceptPXThreshold when a peer sends us PX information with a prune, we only accept it and connect to the supplied
	// peers if the originating peer's penalty exceeds this threshold.
	//
	// Validation Constraint: must be non-negative.
	//
	// How we use it:
	// As current max reward is 100, we set the threshold to 99 so that we only receive supplied peers from
	// well-behaved peers.
	DefaultAcceptPXThreshold = MaxAppSpecificReward - 1

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

	// defaultTopicWeight is the default weight of a topic in the GossipSub scoring system.
	// The overall score of a peer in a topic mesh is multiplied by the weight of the topic when calculating the overall score of the peer.
	// We set it to 1.0, which means that the overall score of a peer in a topic mesh is not affected by the weight of the topic.
	defaultTopicWeight = 1.0

	// defaultTopicMeshMessageDeliveriesDecay is applied to the number of actual message deliveries in a topic mesh
	// at each decay interval (i.e., defaultDecayInterval).
	// It is used to decay the number of actual message deliveries, and prevents past message
	// deliveries from affecting the current score of the peer.
	// As the decay interval is 1 minute, we set it to 0.5, which means that the number of actual message
	// deliveries will decay by 50% at each decay interval.
	defaultTopicMeshMessageDeliveriesDecay = .5

	// defaultTopicMeshMessageDeliveriesCap is the maximum number of actual message deliveries in a topic
	// mesh that is used to calculate the score of a peer in that topic mesh.
	// We set it to 1000, which means that the maximum number of actual message deliveries in a
	// topic mesh that is used to calculate the score of a peer in that topic mesh is 1000.
	// This is to prevent the score of a peer in a topic mesh from being affected by a large number of actual
	// message deliveries and also affect the score of the peer in other topic meshes.
	// When the total delivered messages in a topic mesh exceeds this value, the score of the peer in that topic
	// mesh will not be affected by the actual message deliveries in that topic mesh.
	// Moreover, this does not allow the peer to accumulate a large number of actual message deliveries in a topic mesh
	// and then start under-performing in that topic mesh without being penalized.
	defaultTopicMeshMessageDeliveriesCap = 1000

	// defaultTopicMeshMessageDeliveriesThreshold is the threshold for the number of actual message deliveries in a
	// topic mesh that is used to calculate the score of a peer in that topic mesh.
	// If the number of actual message deliveries in a topic mesh is less than this value,
	// the peer will be penalized by square of the difference between the actual message deliveries and the threshold,
	// i.e., -w * (actual - threshold)^2 where `actual` and `threshold` are the actual message deliveries and the
	// threshold, respectively, and `w` is the weight (i.e., defaultTopicMeshMessageDeliveriesWeight).
	// We set it to 0.1 * defaultTopicMeshMessageDeliveriesCap, which means that if a peer delivers less tha 10% of the
	// maximum number of actual message deliveries in a topic mesh, it will be considered as an under-performing peer
	// in that topic mesh.
	defaultTopicMeshMessageDeliveryThreshold = 0.1 * defaultTopicMeshMessageDeliveriesCap

	// defaultTopicMeshDeliveriesWeight is the weight for applying penalty when a peer is under-performing in a topic mesh.
	// Upon every decay interval, if the number of actual message deliveries is less than the topic mesh message deliveries threshold
	// (i.e., defaultTopicMeshMessageDeliveriesThreshold), the peer will be penalized by square of the difference between the actual
	// message deliveries and the threshold, multiplied by this weight, i.e., -w * (actual - threshold)^2 where w is the weight, and
	// `actual` and `threshold` are the actual message deliveries and the threshold, respectively.
	// We set this value to be - 0.05 MaxAppSpecificReward / (defaultTopicMeshMessageDeliveriesThreshold^2). This guarantees that even if a peer
	// is not delivering any message in a topic mesh, it will not be disconnected.
	// Rather, looses part of the MaxAppSpecificReward that is awarded by our app-specific scoring function to all staked
	// nodes by default will be withdrawn, and the peer will be slightly penalized. In other words, under-performing in a topic mesh
	// will drop the overall score of a peer by 5% of the MaxAppSpecificReward that is awarded by our app-specific scoring function.
	// It means that under-performing in a topic mesh will not cause a peer to be disconnected, but it will cause the peer to lose
	// its MaxAppSpecificReward that is awarded by our app-specific scoring function.
	// At this point, we do not want to disconnect a peer only because it is under-performing in a topic mesh as it might be
	// causing a false positive network partition.
	// TODO: we must increase the penalty for under-performing in a topic mesh in the future, and disconnect the peer if it is under-performing.
	defaultTopicMeshMessageDeliveriesWeight = -0.05 * MaxAppSpecificReward / (defaultTopicMeshMessageDeliveryThreshold * defaultTopicMeshMessageDeliveryThreshold)

	// defaultMeshMessageDeliveriesWindow is the window size is time interval that we count a delivery of an already
	// seen message towards the score of a peer in a topic mesh. The delivery is counted
	// by GossipSub only if the previous sender of the message is different from the current sender.
	// We set it to the decay interval of the GossipSub scoring system, which is 1 minute.
	// It means that if a peer delivers a message that it has already seen less than one minute ago,
	// the delivery will be counted towards the score of the peer in a topic mesh only if the previous sender of the message.
	// This also prevents replay attacks of messages that are older than one minute. As replayed messages will not
	// be counted towards the actual message deliveries of a peer in a topic mesh.
	defaultMeshMessageDeliveriesWindow = defaultDecayInterval

	// defaultMeshMessageDeliveryActivation is the time interval that we wait for a new peer that joins a topic mesh
	// till start counting the number of actual message deliveries of that peer in that topic mesh.
	// We set it to 2 * defaultDecayInterval, which means that we wait for 2 decay intervals before start counting
	// the number of actual message deliveries of a peer in a topic mesh.
	// With a default decay interval of 1 minute, it means that we wait for 2 minutes before start counting the
	// number of actual message deliveries of a peer in a topic mesh. This is to account for
	// the time that it takes for a peer to start up and receive messages from other peers in the topic mesh.
	defaultMeshMessageDeliveriesActivation = 2 * defaultDecayInterval

	// defaultBehaviorPenaltyThreshold is the threshold when the behavior of a peer is considered as bad by GossipSub.
	// Currently, the misbehavior is defined as advertising an iHave without responding to the iWants (iHave broken promises), as well as attempting
	// on GRAFT when the peer is considered for a PRUNE backoff, i.e., the local peer does not allow the peer to join the local topic mesh
	// for a while, and the remote peer keep attempting on GRAFT (aka GRAFT flood).
	// When the misbehavior counter of a peer goes beyond this threshold, the peer is penalized by defaultBehaviorPenaltyWeight (see below) for the excess misbehavior.
	//
	// An iHave broken promise means that a peer advertises an iHave for a message, but does not respond to the iWant requests for that message.
	// For iHave broken promises, the gossipsub scoring works as follows:
	// It samples ONLY A SINGLE iHave out of the entire RPC.
	// If that iHave is not followed by an actual message within the next 3 seconds, the peer misbehavior counter is incremented by 1.
	//
	// We set it to 10, meaning that we at most tolerate 10 of such RPCs containing iHave broken promises. After that, the peer is penalized for every
	// excess RPC containing iHave broken promises.
	// The counter is also decayed by (0.99) every decay interval (defaultDecayInterval) i.e., every minute.
	// Note that misbehaviors are counted by GossipSub across all topics (and is different from the Application Layer Misbehaviors that we count through
	// the ALSP system).
	defaultBehaviourPenaltyThreshold = 10

	// defaultBehaviorPenaltyWeight is the weight for applying penalty when a peer misbehavior goes beyond the threshold.
	// Misbehavior of a peer at gossipsub layer is defined as advertising an iHave without responding to the iWants (broken promises), as well as attempting
	// on GRAFT when the peer is considered for a PRUNE backoff, i.e., the local peer does not allow the peer to join the local topic mesh
	// This is detected by the GossipSub scoring system, and the peer is penalized by defaultBehaviorPenaltyWeight.
	//
	// An iHave broken promise means that a peer advertises an iHave for a message, but does not respond to the iWant requests for that message.
	// For iHave broken promises, the gossipsub scoring works as follows:
	// It samples ONLY A SINGLE iHave out of the entire RPC.
	// If that iHave is not followed by an actual message within the next 3 seconds, the peer misbehavior counter is incremented by 1.
	//
	// The penalty is applied to the square of the difference between the misbehavior counter and the threshold, i.e., -|w| * (misbehavior counter - threshold)^2.
	// We set it to 0.01 * MaxAppSpecificPenalty, which means that misbehaving 10 times more than the threshold (i.e., 10 + 10) will cause the peer to lose
	// its entire AppSpecificReward that is awarded by our app-specific scoring function to all staked (i.e., authorized) nodes by default.
	// Moreover, as the MaxAppSpecificPenalty is -MaxAppSpecificReward, misbehaving sqrt(2) * 10 times more than the threshold will cause the peer score
	// to be dropped below the MaxAppSpecificPenalty, which is also below the GraylistThreshold, and the peer will be graylisted (i.e., disconnected).
	//
	// The math is as follows: -|w| * (misbehavior - threshold)^2 = 0.01 * MaxAppSpecificPenalty * (misbehavior - threshold)^2 < 2 * MaxAppSpecificPenalty
	// if misbehavior > threshold + sqrt(2) * 10.
	// As shown above, with this choice of defaultBehaviorPenaltyWeight, misbehaving sqrt(2) * 10 times more than the threshold will cause the peer score
	// to be dropped below the MaxAppSpecificPenalty, which is also below the GraylistThreshold, and the peer will be graylisted (i.e., disconnected). This weight
	// is chosen in a way that with almost a few misbehaviors more than the threshold, the peer will be graylisted. The rationale relies on the fact that
	// the misbehavior counter is incremented by 1 for each RPC containing one or more broken promises. Hence, it is per RPC, and not per broken promise.
	// Having sqrt(2) * 10 broken promises RPC is a blatant misbehavior, and the peer should be graylisted. With decay interval of 1 minute, and decay value of
	// 0.99 we expect a graylisted node due to borken promises to get back in about 527 minutes, i.e., (0.99)^x * (sqrt(2) * 10)^2 * MaxAppSpecificPenalty > GraylistThreshold
	// where x is the number of decay intervals that the peer is graylisted. As MaxAppSpecificPenalty and GraylistThresholds are close, we can simplify the inequality
	// to (0.99)^x * (sqrt(2) * 10)^2 > 1 --> (0.99)^x * 200 > 1 --> (0.99)^x > 1/200 --> x > log(1/200) / log(0.99) --> x > 527.17 decay intervals, i.e., 527 minutes.
	// Note that misbehaviors are counted by GossipSub across all topics (and is different from the Application Layer Misbehaviors that we count through
	// the ALSP system that are reported by the engines).
	defaultBehaviourPenaltyWeight = 0.01 * MaxAppSpecificPenalty

	// defaultBehaviorPenaltyDecay is the decay interval for the misbehavior counter of a peer. The misbehavior counter is
	// incremented by GossipSub for iHave broken promises or the GRAFT flooding attacks (i.e., each GRAFT received from a remote peer while that peer is on a PRUNE backoff).
	//
	// An iHave broken promise means that a peer advertises an iHave for a message, but does not respond to the iWant requests for that message.
	// For iHave broken promises, the gossipsub scoring works as follows:
	// It samples ONLY A SINGLE iHave out of the entire RPC.
	// If that iHave is not followed by an actual message within the next 3 seconds, the peer misbehavior counter is incremented by 1.
	// This means that regardless of how many iHave broken promises an RPC contains, the misbehavior counter is incremented by 1.
	// That is why we decay the misbehavior counter very slow, as this counter indicates a severe misbehavior.
	//
	// The misbehavior counter is decayed per decay interval (i.e., defaultDecayInterval = 1 minute) by GossipSub.
	// We set it to 0.99, which means that the misbehavior counter is decayed by 1% per decay interval.
	// With the generous threshold that we set (i.e., defaultBehaviourPenaltyThreshold = 10), we take the peers going beyond the threshold as persistent misbehaviors,
	// We expect honest peers never to go beyond the threshold, and if they do, we expect them to go back below the threshold quickly.
	//
	// Note that misbehaviors are counted by GossipSub across all topics (and is different from the Application Layer Misbehaviors that we count through
	// the ALSP system that is based on the engines report).
	defaultBehaviourPenaltyDecay = 0.99
)

// ScoreOption is a functional option for configuring the peer scoring system.
// TODO: rename it to ScoreManager.
type ScoreOption struct {
	component.Component
	logger zerolog.Logger

	peerScoreParams     *pubsub.PeerScoreParams
	peerThresholdParams *pubsub.PeerScoreThresholds
	validator           p2p.SubscriptionValidator
	appScoreFunc        func(peer.ID) float64
}

type ScoreOptionConfig struct {
	logger                           zerolog.Logger
	params                           p2pconf.ScoringParameters
	provider                         module.IdentityProvider
	heroCacheMetricsFactory          metrics.HeroCacheMetricsFactory
	appScoreFunc                     func(peer.ID) float64
	topicParams                      []func(map[string]*pubsub.TopicScoreParams)
	registerNotificationConsumerFunc func(p2p.GossipSubInvCtrlMsgNotifConsumer)
}

// NewScoreOptionConfig creates a new configuration for the GossipSub peer scoring option.
// Args:
// - logger: the logger to use.
// - hcMetricsFactory: HeroCache metrics factory to create metrics for the scoring-related caches.
// - idProvider: the identity provider to use.
// Returns:
// - a new configuration for the GossipSub peer scoring option.
func NewScoreOptionConfig(logger zerolog.Logger,
	params p2pconf.ScoringParameters,
	hcMetricsFactory metrics.HeroCacheMetricsFactory,
	idProvider module.IdentityProvider) *ScoreOptionConfig {
	return &ScoreOptionConfig{
		logger:                  logger.With().Str("module", "pubsub_score_option").Logger(),
		provider:                idProvider,
		params:                  params,
		heroCacheMetricsFactory: hcMetricsFactory,
		topicParams:             make([]func(map[string]*pubsub.TopicScoreParams), 0),
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
	throttledSampler := logging.BurstSampler(MaxDebugLogs, time.Second)
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
		Penalty:                 DefaultGossipSubCtrlMsgPenaltyValue(),
		Validator:               validator,
		Init:                    InitAppScoreRecordState,
		IdProvider:              cfg.provider,
		HeroCacheMetricsFactory: cfg.heroCacheMetricsFactory,
		AppScoreCacheFactory: func() p2p.GossipSubApplicationSpecificScoreCache {
			return internal.NewAppSpecificScoreCache(cfg.params.SpamRecordCache.CacheSize, cfg.logger, cfg.heroCacheMetricsFactory)
		},
		SpamRecordCacheFactory: func() p2p.GossipSubSpamRecordCache {
			return netcache.NewGossipSubSpamRecordCache(cfg.params.SpamRecordCache.CacheSize, cfg.logger, cfg.heroCacheMetricsFactory,
				DefaultDecayFunction(
					cfg.params.SpamRecordCache.PenaltyDecaySlowdownThreshold,
					cfg.params.SpamRecordCache.DecayRateReductionFactor,
					cfg.params.SpamRecordCache.PenaltyDecayEvaluationPeriod))
		},
		Parameters: cfg.params.AppSpecificScore,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gossipsub app specific score registry: %w", err)
	}

	s := &ScoreOption{
		logger:          logger,
		validator:       validator,
		peerScoreParams: defaultPeerScoreParams(),
		appScoreFunc:    scoreRegistry.AppSpecificScoreFunc(),
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

	if cfg.params.DecayInterval > 0 && cfg.params.DecayInterval != s.peerScoreParams.DecayInterval {
		// overrides the default decay interval if the decay interval is set.
		s.peerScoreParams.DecayInterval = cfg.params.DecayInterval
		s.logger.
			Warn().
			Str(logging.KeyNetworkingSecurity, "true").
			Dur("decay_interval_ms", cfg.params.DecayInterval).
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
		return DefaultTopicScoreParams()
	}
	return params
}

func defaultPeerScoreParams() *pubsub.PeerScoreParams {
	// DO NOT CHANGE THE DEFAULT VALUES, THEY ARE TUNED FOR THE BEST SECURITY PRACTICES.
	return &pubsub.PeerScoreParams{
		Topics: make(map[string]*pubsub.TopicScoreParams),
		// we don't set all the parameters, so we skip the atomic validation.
		// atomic validation fails initialization if any parameter is not set.
		SkipAtomicValidation: true,
		// DecayInterval is the interval over which we decay the effect of past behavior, so that
		// a good or bad behavior will not have a permanent effect on the penalty. It is also the interval
		// that GossipSub uses to refresh the scores of all peers.
		DecayInterval: defaultDecayInterval,
		// DecayToZero defines the maximum value below which a peer scoring counter is reset to zero.
		// This is to prevent the counter from decaying to a very small value.
		// When a counter hits the DecayToZero threshold, it means that the peer did not exhibit the behavior
		// for a long time, and we can reset the counter.
		DecayToZero: defaultDecayToZero,
		// AppSpecificWeight is the weight of the application specific penalty.
		AppSpecificWeight: DefaultAppSpecificScoreWeight,
		// BehaviourPenaltyThreshold is the threshold above which a peer is penalized for GossipSub-level misbehaviors.
		BehaviourPenaltyThreshold: defaultBehaviourPenaltyThreshold,
		// BehaviourPenaltyWeight is the weight of the GossipSub-level penalty.
		BehaviourPenaltyWeight: defaultBehaviourPenaltyWeight,
		// BehaviourPenaltyDecay is the decay of the GossipSub-level penalty (applied every decay interval).
		BehaviourPenaltyDecay: defaultBehaviourPenaltyDecay,
	}
}

// DefaultTopicScoreParams returns the default score params for topics.
func DefaultTopicScoreParams() *pubsub.TopicScoreParams {
	// DO NOT CHANGE THE DEFAULT VALUES, THEY ARE TUNED FOR THE BEST SECURITY PRACTICES.
	p := &pubsub.TopicScoreParams{
		TopicWeight:                     defaultTopicWeight,
		SkipAtomicValidation:            defaultTopicSkipAtomicValidation,
		InvalidMessageDeliveriesWeight:  defaultTopicInvalidMessageDeliveriesWeight,
		InvalidMessageDeliveriesDecay:   defaultTopicInvalidMessageDeliveriesDecay,
		TimeInMeshQuantum:               defaultTopicTimeInMesh,
		MeshMessageDeliveriesWeight:     defaultTopicMeshMessageDeliveriesWeight,
		MeshMessageDeliveriesDecay:      defaultTopicMeshMessageDeliveriesDecay,
		MeshMessageDeliveriesCap:        defaultTopicMeshMessageDeliveriesCap,
		MeshMessageDeliveriesThreshold:  defaultTopicMeshMessageDeliveryThreshold,
		MeshMessageDeliveriesWindow:     defaultMeshMessageDeliveriesWindow,
		MeshMessageDeliveriesActivation: defaultMeshMessageDeliveriesActivation,
	}

	if p.MeshMessageDeliveriesWeight >= 0 {
		// GossipSub also does a validation, but we want to panic as early as possible.
		panic(fmt.Sprintf("invalid mesh message deliveries weight %f", p.MeshMessageDeliveriesWeight))
	}

	return p
}
