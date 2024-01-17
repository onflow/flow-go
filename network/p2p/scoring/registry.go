package scoring

import (
	"fmt"
	"math"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/common/worker"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	netcache "github.com/onflow/flow-go/network/p2p/cache"
	p2pconfig "github.com/onflow/flow-go/network/p2p/config"
	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	"github.com/onflow/flow-go/utils/logging"
)

type SpamRecordInitFunc func() p2p.GossipSubSpamRecord

// GossipSubAppSpecificScoreRegistry is the registry for the application specific score of peers in the GossipSub protocol.
// The application specific score is part of the overall score of a peer, and is used to determine the peer's score based
// on its behavior related to the application (Flow protocol).
// This registry holds the view of the local peer of the application specific score of other peers in the network based
// on what it has observed from the network.
// Similar to the GossipSub score, the application specific score is meant to be private to the local peer, and is not
// shared with other peers in the network.
type GossipSubAppSpecificScoreRegistry struct {
	component.Component
	logger     zerolog.Logger
	idProvider module.IdentityProvider

	// spamScoreCache currently only holds the control message misbehaviour penalty (spam related penalty).
	spamScoreCache p2p.GossipSubSpamRecordCache

	penalty p2pconfig.MisbehaviourPenalties

	validator p2p.SubscriptionValidator

	// scoreTTL is the time to live of the application specific score of a peer; the registry keeps a cached copy of the
	// application specific score of a peer for this duration. When the duration expires, the application specific score
	// of the peer is updated asynchronously. As long as the update is in progress, the cached copy of the application
	// specific score of the peer is used even if it is expired.
	scoreTTL time.Duration

	// appScoreCache is a cache that stores the application specific score of peers.
	appScoreCache p2p.GossipSubApplicationSpecificScoreCache

	// appScoreUpdateWorkerPool is the worker pool for handling the application specific score update of peers in a non-blocking way.
	appScoreUpdateWorkerPool *worker.Pool[peer.ID]

	unknownIdentityPenalty     float64
	minAppSpecificPenalty      float64
	stakedIdentityReward       float64
	invalidSubscriptionPenalty float64
}

// GossipSubAppSpecificScoreRegistryConfig is the configuration for the GossipSubAppSpecificScoreRegistry.
// Configurations are the "union of parameters and other components" that are used to compute or build components that compute or maintain the application specific score of peers.
type GossipSubAppSpecificScoreRegistryConfig struct {
	Parameters p2pconfig.AppSpecificScoreParameters `validate:"required"`

	Logger zerolog.Logger `validate:"required"`

	// Validator is the subscription validator used to validate the subscriptions of peers, and determine if a peer is
	// authorized to subscribe to a topic.
	Validator p2p.SubscriptionValidator `validate:"required"`

	// Penalty encapsulates the penalty unit for each control message type misbehaviour.
	Penalty p2pconfig.MisbehaviourPenalties `validate:"required"`

	// IdProvider is the identity provider used to translate peer ids at the networking layer to Flow identifiers (if
	// an authorized peer is found).
	IdProvider module.IdentityProvider `validate:"required"`

	// SpamRecordCacheFactory is a factory function that returns a new GossipSubSpamRecordCache. It is used to initialize the spamScoreCache.
	// The cache is used to store the application specific penalty of peers.
	SpamRecordCacheFactory func() p2p.GossipSubSpamRecordCache `validate:"required"`

	// AppScoreCacheFactory is a factory function that returns a new GossipSubApplicationSpecificScoreCache. It is used to initialize the appScoreCache.
	// The cache is used to store the application specific score of peers.
	AppScoreCacheFactory func() p2p.GossipSubApplicationSpecificScoreCache `validate:"required"`

	HeroCacheMetricsFactory metrics.HeroCacheMetricsFactory `validate:"required"`

	NetworkingType network.NetworkingType `validate:"required"`

	AppSpecificScoreParams p2pconfig.ApplicationSpecificScoreParameters `validate:"required"`
}

// NewGossipSubAppSpecificScoreRegistry returns a new GossipSubAppSpecificScoreRegistry.
// Args:
//
//	config: the config for the registry.
//
// Returns:
//
//	a new GossipSubAppSpecificScoreRegistry.
//
// error: if the configuration is invalid, an error is returned; any returned error is an irrecoverable error and indicates a bug or misconfiguration.
func NewGossipSubAppSpecificScoreRegistry(config *GossipSubAppSpecificScoreRegistryConfig) (*GossipSubAppSpecificScoreRegistry, error) {
	if err := validator.New().Struct(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	lg := config.Logger.With().Str("module", "app_score_registry").Logger()
	store := queue.NewHeroStore(config.Parameters.ScoreUpdateRequestQueueSize,
		lg.With().Str("component", "app_specific_score_update").Logger(),
		metrics.GossipSubAppSpecificScoreUpdateQueueMetricFactory(config.HeroCacheMetricsFactory, config.NetworkingType))

	reg := &GossipSubAppSpecificScoreRegistry{
		logger:                     config.Logger.With().Str("module", "app_score_registry").Logger(),
		spamScoreCache:             config.SpamRecordCacheFactory(),
		appScoreCache:              config.AppScoreCacheFactory(),
		penalty:                    config.Penalty,
		validator:                  config.Validator,
		idProvider:                 config.IdProvider,
		scoreTTL:                   config.Parameters.ScoreTTL,
		unknownIdentityPenalty:     config.AppSpecificScoreParams.UnknownIdentityPenalty,
		minAppSpecificPenalty:      config.AppSpecificScoreParams.MinAppSpecificPenalty,
		stakedIdentityReward:       config.AppSpecificScoreParams.StakedIdentityReward,
		invalidSubscriptionPenalty: config.AppSpecificScoreParams.InvalidSubscriptionPenalty,
	}

	reg.appScoreUpdateWorkerPool = worker.NewWorkerPoolBuilder[peer.ID](lg.With().Str("component", "app_specific_score_update_worker_pool").Logger(),
		store,
		reg.processAppSpecificScoreUpdateWork).Build()

	builder := component.NewComponentManagerBuilder()
	builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		reg.logger.Info().Msg("starting subscription validator")
		reg.validator.Start(ctx)
		select {
		case <-ctx.Done():
			reg.logger.Warn().Msg("aborting subscription validator startup, context cancelled")
		case <-reg.validator.Ready():
			reg.logger.Info().Msg("subscription validator started")
			ready()
			reg.logger.Info().Msg("subscription validator is ready")
		}

		<-ctx.Done()
		reg.logger.Info().Msg("stopping subscription validator")
		<-reg.validator.Done()
		reg.logger.Info().Msg("subscription validator stopped")
	})

	for i := 0; i < config.Parameters.ScoreUpdateWorkerNum; i++ {
		builder.AddWorker(reg.appScoreUpdateWorkerPool.WorkerLogic())
	}

	reg.Component = builder.Build()

	return reg, nil
}

var _ p2p.GossipSubInvCtrlMsgNotifConsumer = (*GossipSubAppSpecificScoreRegistry)(nil)

// AppSpecificScoreFunc returns the application specific score function that is called by the GossipSub protocol to determine the application specific score of a peer.
// The application specific score is part of the overall score of a peer, and is used to determine the peer's score based
// This function reads the application specific score of a peer from the cache, and if the penalty is not found in the cache, it computes it.
// If the score is not found in the cache, it is computed and added to the cache.
// Also if the score is expired, it is computed and added to the cache.
// Returns:
// - func(peer.ID) float64: the application specific score function.
// Implementation must be thread-safe.
func (r *GossipSubAppSpecificScoreRegistry) AppSpecificScoreFunc() func(peer.ID) float64 {
	return func(pid peer.ID) float64 {
		lg := r.logger.With().Str("remote_peer_id", p2plogging.PeerId(pid)).Logger()
		appSpecificScore, lastUpdated, ok := r.appScoreCache.Get(pid)
		switch {
		case !ok:
			// record not found in the cache, or expired; submit a worker to update it.
			submitted := r.appScoreUpdateWorkerPool.Submit(pid)
			lg.Trace().
				Bool("worker_submitted", submitted).
				Msg("application specific score not found in cache, submitting worker to update it")
			return 0 // in the mean time, return 0, which is a neutral score.
		case time.Since(lastUpdated) > r.scoreTTL:
			// record found in the cache, but expired; submit a worker to update it.
			submitted := r.appScoreUpdateWorkerPool.Submit(pid)
			lg.Trace().
				Bool("worker_submitted", submitted).
				Float64("app_specific_score", appSpecificScore).
				Dur("score_ttl", r.scoreTTL).
				Msg("application specific score expired, submitting worker to update it")

			return appSpecificScore // in the mean time, return the expired score.
		default:
			// record found in the cache.
			r.logger.Trace().
				Float64("app_specific_score", appSpecificScore).
				Msg("application specific score found in cache")
			return appSpecificScore
		}
	}
}

// computeAppSpecificScore computes the application specific score of a peer.
// The application specific score is computed based on the spam penalty, staking score, and subscription penalty.
// The spam penalty is the penalty applied to the application specific score when a peer conducts a spamming misbehaviour.
// The staking score is the reward/penalty applied to the application specific score when a peer is staked/unstaked.
// The subscription penalty is the penalty applied to the application specific score when a peer is subscribed to a topic that it is not allowed to subscribe to based on its role.
// Args:
// - pid: the peer ID of the peer in the GossipSub protocol.
// Returns:
// - float64: the application specific score of the peer.
func (r *GossipSubAppSpecificScoreRegistry) computeAppSpecificScore(pid peer.ID) float64 {
	appSpecificScore := float64(0)
	lg := r.logger.With().Str("peer_id", p2plogging.PeerId(pid)).Logger()
	// (1) spam penalty: the penalty is applied to the application specific penalty when a peer conducts a spamming misbehaviour.
	spamRecord, err, spamRecordExists := r.spamScoreCache.Get(pid)
	if err != nil {
		// the error is considered fatal as it means the cache is not working properly.
		// we should not continue with the execution as it may lead to routing attack vulnerability.
		r.logger.Fatal().Str("peer_id", p2plogging.PeerId(pid)).Err(err).Msg("could not get application specific penalty for peer")
		return appSpecificScore // unreachable, but added to avoid proceeding with the execution if log level is changed.
	}

	if spamRecordExists {
		lg = lg.With().Float64("spam_penalty", spamRecord.Penalty).Logger()
		appSpecificScore += spamRecord.Penalty
	}

	// (2) staking score: for staked peers, a default positive reward is applied only if the peer has no penalty on spamming and subscription.
	// for unknown peers a negative penalty is applied.
	stakingScore, flowId, role := r.stakingScore(pid)
	if stakingScore < 0 {
		lg = lg.With().Float64("staking_penalty", stakingScore).Logger()
		// staking penalty is applied right away.
		appSpecificScore += stakingScore
	}

	if stakingScore >= 0 {
		// (3) subscription penalty: the subscription penalty is applied to the application specific penalty when a
		// peer is subscribed to a topic that it is not allowed to subscribe to based on its role.
		// Note: subscription penalty can be considered only for staked peers, for non-staked peers, we cannot
		// determine the role of the peer.
		subscriptionPenalty := r.subscriptionPenalty(pid, flowId, role)
		lg = lg.With().Float64("subscription_penalty", subscriptionPenalty).Logger()
		if subscriptionPenalty < 0 {
			appSpecificScore += subscriptionPenalty
		}
	}

	// (4) staking reward: for staked peers, a default positive reward is applied only if the peer has no penalty on spamming and subscription.
	if stakingScore > 0 && appSpecificScore == float64(0) {
		lg = lg.With().Float64("staking_reward", stakingScore).Logger()
		appSpecificScore += stakingScore
	}

	lg.Trace().
		Float64("total_app_specific_score", appSpecificScore).
		Msg("application specific score computed")
	return appSpecificScore
}

// processMisbehaviorReport is the worker function that is called by the worker pool to update the application specific score of a peer.
// The function is called in a non-blocking way, and the worker pool is used to limit the number of concurrent executions of the function.
// Args:
// - pid: the peer ID of the peer in the GossipSub protocol.
// Returns:
// - error: an error if the update failed; any returned error is an irrecoverable error and indicates a bug or misconfiguration.
func (r *GossipSubAppSpecificScoreRegistry) processAppSpecificScoreUpdateWork(p peer.ID) error {
	appSpecificScore := r.computeAppSpecificScore(p)
	err := r.appScoreCache.AdjustWithInit(p, appSpecificScore, time.Now())
	if err != nil {
		// the error is considered fatal as it means the cache is not working properly.
		return fmt.Errorf("could not add application specific score %f for peer to cache: %w", appSpecificScore, err)
	}
	r.logger.Trace().
		Str("remote_peer_id", p2plogging.PeerId(p)).
		Float64("app_specific_score", appSpecificScore).
		Msg("application specific score computed and cache updated")
	return nil
}

func (r *GossipSubAppSpecificScoreRegistry) stakingScore(pid peer.ID) (float64, flow.Identifier, flow.Role) {
	lg := r.logger.With().Str("peer_id", p2plogging.PeerId(pid)).Logger()

	// checks if peer has a valid Flow protocol identity.
	flowId, err := HasValidFlowIdentity(r.idProvider, pid)
	if err != nil {
		lg.Error().
			Err(err).
			Bool(logging.KeySuspicious, true).
			Msg("invalid peer identity, penalizing peer")
		return r.unknownIdentityPenalty, flow.Identifier{}, 0
	}

	lg = lg.With().
		Hex("flow_id", logging.ID(flowId.NodeID)).
		Str("role", flowId.Role.String()).
		Logger()

	// checks if peer is an access node, and if so, pushes it to the
	// edges of the network by giving the minimum penalty.
	if flowId.Role == flow.RoleAccess {
		lg.Trace().
			Msg("pushing access node to edge by penalizing with minimum penalty value")
		return r.minAppSpecificPenalty, flowId.NodeID, flowId.Role
	}

	lg.Trace().
		Msg("rewarding well-behaved non-access node peer with maximum reward value")

	return r.stakedIdentityReward, flowId.NodeID, flowId.Role
}

func (r *GossipSubAppSpecificScoreRegistry) subscriptionPenalty(pid peer.ID, flowId flow.Identifier, role flow.Role) float64 {
	// checks if peer has any subscription violation.
	if err := r.validator.CheckSubscribedToAllowedTopics(pid, role); err != nil {
		r.logger.Warn().
			Err(err).
			Str("peer_id", p2plogging.PeerId(pid)).
			Hex("flow_id", logging.ID(flowId)).
			Bool(logging.KeySuspicious, true).
			Msg("invalid subscription detected, penalizing peer")
		return r.invalidSubscriptionPenalty
	}

	return 0
}

// OnInvalidControlMessageNotification is called when a new invalid control message notification is distributed.
// Any error on consuming event must handle internally.
// The implementation must be concurrency safe, but can be blocking.
func (r *GossipSubAppSpecificScoreRegistry) OnInvalidControlMessageNotification(notification *p2p.InvCtrlMsgNotif) {
	// we use mutex to ensure the method is concurrency safe.

	lg := r.logger.With().
		Err(notification.Errors.Error()).
		Str("peer_id", p2plogging.PeerId(notification.PeerID)).
		Str("misbehavior_type", notification.MsgType.String()).Logger()

	record, err := r.spamScoreCache.Adjust(notification.PeerID, func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		basePenalty := float64(0)
		switch notification.MsgType {
		case p2pmsg.CtrlMsgGraft:
			basePenalty += r.penalty.GraftMisbehaviour
		case p2pmsg.CtrlMsgPrune:
			basePenalty += r.penalty.PruneMisbehaviour
		case p2pmsg.CtrlMsgIHave:
			basePenalty += r.penalty.IHaveMisbehaviour
		case p2pmsg.CtrlMsgIWant:
			basePenalty += r.penalty.IWantMisbehaviour
		case p2pmsg.RpcPublishMessage:
			basePenalty += r.penalty.PublishMisbehaviour
		default:
			// the error is considered fatal as it means that we have an unsupported misbehaviour type, we should crash the node to prevent routing attack vulnerability.
			lg.Fatal().Str("misbehavior_type", notification.MsgType.String()).Msg("unknown misbehaviour type")
		}

		// apply the base penalty to the application specific penalty.
		appliedPenalty := float64(0)
		for _, err := range notification.Errors {
			errPenalty := basePenalty
			// reduce penalty for cluster prefixed topics allowing nodes that are potentially behind to catch up
			if err.CtrlMsgTopicType() == p2p.CtrlMsgTopicTypeClusterPrefixed {
				errPenalty *= r.penalty.ClusterPrefixedReductionFactor
			}
			appliedPenalty += errPenalty
		}

		record.Penalty += appliedPenalty

		lg.Debug().
			Err(notification.Errors.Error()).
			Str("control_message_type", notification.MsgType.String()).
			Bool(logging.KeySuspicious, true).
			Bool(logging.KeyNetworkingSecurity, true).
			Int("error_count", notification.Errors.Len()).
			Float64("applied_penalty", appliedPenalty).
			Float64("app_specific_score", record.Penalty).
			Msg("processed invalid control message notification, applied misbehaviour penalty and updated application specific penalty ")

		return record
	})
	if err != nil {
		// any returned error from adjust is non-recoverable and fatal, we crash the node.
		lg.Fatal().Err(err).Msg("could not adjust application specific penalty for peer")
	}

	lg.Debug().
		Float64("spam_record_penalty", record.Penalty).
		Msg("applied misbehaviour penalty and updated application specific penalty")

}

// DefaultDecayFunction is the default decay function that is used to decay the application specific penalty of a peer.
// It is used if no decay function is provided in the configuration.
// It decays the application specific penalty of a peer if it is negative.
func DefaultDecayFunction(cfg p2pconfig.SpamRecordCacheDecay) netcache.PreprocessorFunc {
	return func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
		if record.Penalty >= 0 {
			// no need to decay the penalty if it is positive, the reason is currently the app specific penalty
			// is only used to penalize peers. Hence, when there is no reward, there is no need to decay the positive penalty, as
			// no node can accumulate a positive penalty.
			return record, nil
		}

		if record.Penalty > cfg.SkipDecayThreshold {
			// penalty is negative but greater than the threshold, we set it to 0.
			record.Penalty = 0
			record.Decay = cfg.MaximumSpamPenaltyDecayFactor
			record.LastDecayAdjustment = time.Time{}
			return record, nil
		}

		// penalty is negative and below the threshold, we decay it.
		penalty, err := GeometricDecay(record.Penalty, record.Decay, lastUpdated)
		if err != nil {
			return record, fmt.Errorf("could not decay application specific penalty: %w", err)
		}
		record.Penalty = penalty

		if record.Penalty <= cfg.PenaltyDecaySlowdownThreshold {
			if time.Since(record.LastDecayAdjustment) > cfg.PenaltyDecayEvaluationPeriod || record.LastDecayAdjustment.IsZero() {
				// reduces the decay speed flooring at MinimumSpamRecordDecaySpeed
				record.Decay = math.Min(record.Decay+cfg.DecayRateReductionFactor, cfg.MinimumSpamPenaltyDecayFactor)
				record.LastDecayAdjustment = time.Now()
			}
		}
		return record, nil
	}
}

// InitAppScoreRecordStateFunc returns a callback that initializes the gossipsub spam record state for a peer.
// Returns:
//   - a func that returns a gossipsub spam record with the default decay value and 0 penalty.
func InitAppScoreRecordStateFunc(maximumSpamPenaltyDecayFactor float64) func() p2p.GossipSubSpamRecord {
	return func() p2p.GossipSubSpamRecord {
		return p2p.GossipSubSpamRecord{
			Decay:               maximumSpamPenaltyDecayFactor,
			Penalty:             0,
			LastDecayAdjustment: time.Now(),
		}
	}
}
