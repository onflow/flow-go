package scoring

import (
	"fmt"
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
	"github.com/onflow/flow-go/network/p2p"
	netcache "github.com/onflow/flow-go/network/p2p/cache"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// skipDecayThreshold is the threshold for which when the negative penalty is above this value, the decay function will not be called.
	// instead, the penalty will be set to 0. This is to prevent the penalty from keeping a small negative value for a long time.
	skipDecayThreshold = -0.1
	// defaultDecay is the default decay value for the application specific penalty.
	// this value is used when no custom decay value is provided, and  decays the penalty by 1% every second.
	// assume:
	//     penalty = -100 (the maximum application specific penalty is -100)
	//     skipDecayThreshold = -0.1
	// it takes around 459 seconds for the penalty to decay to reach greater than -0.1 and turn into 0.
	//     x * 0.99 ^ n > -0.1 (assuming negative x).
	//     0.99 ^ n > -0.1 / x
	// Now we can take the logarithm of both sides (with any base, but let's use base 10 for simplicity).
	//     log( 0.99 ^ n ) < log( 0.1 / x )
	// Using the properties of logarithms, we can bring down the exponent:
	//     n * log( 0.99 ) < log( -0.1 / x )
	// And finally, we can solve for n:
	//     n > log( -0.1 / x ) / log( 0.99 )
	// We can plug in x = -100:
	//     n > log( -0.1 / -100 ) / log( 0.99 )
	//     n > log( 0.001 ) / log( 0.99 )
	//     n > -3 / log( 0.99 )
	//     n >  458.22
	defaultDecay = 0.99 // default decay value for the application specific penalty.
	// graftMisbehaviourPenalty is the penalty applied to the application specific penalty when a peer conducts a graft misbehaviour.
	graftMisbehaviourPenalty = -10
	// pruneMisbehaviourPenalty is the penalty applied to the application specific penalty when a peer conducts a prune misbehaviour.
	pruneMisbehaviourPenalty = -10
	// iHaveMisbehaviourPenalty is the penalty applied to the application specific penalty when a peer conducts a iHave misbehaviour.
	iHaveMisbehaviourPenalty = -10
	// iWantMisbehaviourPenalty is the penalty applied to the application specific penalty when a peer conducts a iWant misbehaviour.
	iWantMisbehaviourPenalty = -10
	// rpcPublishMessageMisbehaviourPenalty is the penalty applied to the application specific penalty when a peer conducts a RpcPublishMessageMisbehaviourPenalty misbehaviour.
	rpcPublishMessageMisbehaviourPenalty = -10
)

// GossipSubCtrlMsgPenaltyValue is the penalty value for each control message type.
type GossipSubCtrlMsgPenaltyValue struct {
	Graft             float64 // penalty value for an individual graft message misbehaviour.
	Prune             float64 // penalty value for an individual prune message misbehaviour.
	IHave             float64 // penalty value for an individual iHave message misbehaviour.
	IWant             float64 // penalty value for an individual iWant message misbehaviour.
	RpcPublishMessage float64 // penalty value for an individual RpcPublishMessage message misbehaviour.
}

// DefaultGossipSubCtrlMsgPenaltyValue returns the default penalty value for each control message type.
func DefaultGossipSubCtrlMsgPenaltyValue() GossipSubCtrlMsgPenaltyValue {
	return GossipSubCtrlMsgPenaltyValue{
		Graft:             graftMisbehaviourPenalty,
		Prune:             pruneMisbehaviourPenalty,
		IHave:             iHaveMisbehaviourPenalty,
		IWant:             iWantMisbehaviourPenalty,
		RpcPublishMessage: rpcPublishMessageMisbehaviourPenalty,
	}
}

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

	penalty GossipSubCtrlMsgPenaltyValue

	// initial application specific penalty record, used to initialize the penalty cache entry.
	init func() p2p.GossipSubSpamRecord

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
}

// GossipSubAppSpecificScoreRegistryConfig is the configuration for the GossipSubAppSpecificScoreRegistry.
// Configurations are the "union of parameters and other components" that are used to compute or build components that compute or maintain the application specific score of peers.
type GossipSubAppSpecificScoreRegistryConfig struct {
	Parameters p2pconf.AppSpecificScoreRegistryParams `validate:"required"`

	Logger zerolog.Logger `validate:"required"`

	// Validator is the subscription validator used to validate the subscriptions of peers, and determine if a peer is
	// authorized to subscribe to a topic.
	Validator p2p.SubscriptionValidator `validate:"required"`

	// Penalty encapsulates the penalty unit for each control message type misbehaviour.
	Penalty GossipSubCtrlMsgPenaltyValue `validate:"required"`

	// IdProvider is the identity provider used to translate peer ids at the networking layer to Flow identifiers (if
	// an authorized peer is found).
	IdProvider module.IdentityProvider `validate:"required"`

	// Init is a factory function that returns a new GossipSubSpamRecord. It is used to initialize the spam record of
	// a peer when the peer is first observed by the local peer.
	Init func() p2p.GossipSubSpamRecord `validate:"required"`

	// SpamRecordCacheFactory is a factory function that returns a new GossipSubSpamRecordCache. It is used to initialize the spamScoreCache.
	// The cache is used to store the application specific penalty of peers.
	SpamRecordCacheFactory func() p2p.GossipSubSpamRecordCache `validate:"required"`

	// AppScoreCacheFactory is a factory function that returns a new GossipSubApplicationSpecificScoreCache. It is used to initialize the appScoreCache.
	// The cache is used to store the application specific score of peers.
	AppScoreCacheFactory func() p2p.GossipSubApplicationSpecificScoreCache `validate:"required"`

	HeroCacheMetricsFactory metrics.HeroCacheMetricsFactory `validate:"required"`
}

// NewGossipSubAppSpecificScoreRegistry returns a new GossipSubAppSpecificScoreRegistry.
// Args:
//
//	config: the configuration for the registry.
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
		metrics.GossipSubAppSpecificScoreUpdateQueueMetricFactory(config.HeroCacheMetricsFactory))

	reg := &GossipSubAppSpecificScoreRegistry{
		logger:         config.Logger.With().Str("module", "app_score_registry").Logger(),
		spamScoreCache: config.SpamRecordCacheFactory(),
		appScoreCache:  config.AppScoreCacheFactory(),
		penalty:        config.Penalty,
		init:           config.Init,
		validator:      config.Validator,
		idProvider:     config.IdProvider,
		scoreTTL:       config.Parameters.ScoreTTL,
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
			// record found in the cache, check if it is expired.
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
		Msg("application specific penalty computed")

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
	err := r.appScoreCache.Add(p, appSpecificScore, time.Now())
	if err != nil {
		// the error is considered fatal as it means the cache is not working properly.
		return fmt.Errorf("could not add application specific score %f for peer to cache: %w", appSpecificScore, err)
	}
	r.logger.Trace().
		Str("remote_peer_id", p2plogging.PeerId(p)).
		Float64("app_specific_score", appSpecificScore).
		Msg("application specific penalty computed and cache updated")
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
		return DefaultUnknownIdentityPenalty, flow.Identifier{}, 0
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
		return MinAppSpecificPenalty, flowId.NodeID, flowId.Role
	}

	lg.Trace().
		Msg("rewarding well-behaved non-access node peer with maximum reward value")

	return DefaultStakedIdentityReward, flowId.NodeID, flowId.Role
}

func (r *GossipSubAppSpecificScoreRegistry) subscriptionPenalty(pid peer.ID, flowId flow.Identifier, role flow.Role) float64 {
	// checks if peer has any subscription violation.
	if err := r.validator.CheckSubscribedToAllowedTopics(pid, role); err != nil {
		r.logger.Err(err).
			Str("peer_id", p2plogging.PeerId(pid)).
			Hex("flow_id", logging.ID(flowId)).
			Bool(logging.KeySuspicious, true).
			Msg("invalid subscription detected, penalizing peer")
		return DefaultInvalidSubscriptionPenalty
	}

	return 0
}

// OnInvalidControlMessageNotification is called when a new invalid control message notification is distributed.
// Any error on consuming event must handle internally.
// The implementation must be concurrency safe, but can be blocking.
func (r *GossipSubAppSpecificScoreRegistry) OnInvalidControlMessageNotification(notification *p2p.InvCtrlMsgNotif) {
	// we use mutex to ensure the method is concurrency safe.

	lg := r.logger.With().
		Err(notification.Error).
		Str("peer_id", p2plogging.PeerId(notification.PeerID)).
		Str("misbehavior_type", notification.MsgType.String()).Logger()

	// try initializing the application specific penalty for the peer if it is not yet initialized.
	// this is done to avoid the case where the peer is not yet cached and the application specific penalty is not yet initialized.
	// initialization is successful only if the peer is not yet cached.
	initialized := r.spamScoreCache.Add(notification.PeerID, r.init())
	if initialized {
		lg.Trace().Str("peer_id", p2plogging.PeerId(notification.PeerID)).Msg("application specific penalty initialized for peer")
	}

	record, err := r.spamScoreCache.Update(notification.PeerID, func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		switch notification.MsgType {
		case p2pmsg.CtrlMsgGraft:
			record.Penalty += r.penalty.Graft
		case p2pmsg.CtrlMsgPrune:
			record.Penalty += r.penalty.Prune
		case p2pmsg.CtrlMsgIHave:
			record.Penalty += r.penalty.IHave
		case p2pmsg.CtrlMsgIWant:
			record.Penalty += r.penalty.IWant
		case p2pmsg.RpcPublishMessage:
			record.Penalty += r.penalty.RpcPublishMessage
		default:
			// the error is considered fatal as it means that we have an unsupported misbehaviour type, we should crash the node to prevent routing attack vulnerability.
			lg.Fatal().Str("misbehavior_type", notification.MsgType.String()).Msg("unknown misbehaviour type")
		}
		return record
	})
	if err != nil {
		// any returned error from adjust is non-recoverable and fatal, we crash the node.
		lg.Fatal().Err(err).Msg("could not adjust application specific penalty for peer")
	}

	lg.Debug().
		Float64("app_specific_score", record.Penalty).
		Msg("applied misbehaviour penalty and updated application specific penalty")
}

// DefaultDecayFunction is the default decay function that is used to decay the application specific penalty of a peer.
// It is used if no decay function is provided in the configuration.
// It decays the application specific penalty of a peer if it is negative.
func DefaultDecayFunction() netcache.PreprocessorFunc {
	return func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
		if record.Penalty >= 0 {
			// no need to decay the penalty if it is positive, the reason is currently the app specific penalty
			// is only used to penalize peers. Hence, when there is no reward, there is no need to decay the positive penalty, as
			// no node can accumulate a positive penalty.
			return record, nil
		}

		if record.Penalty > skipDecayThreshold {
			// penalty is negative but greater than the threshold, we set it to 0.
			record.Penalty = 0
			return record, nil
		}

		// penalty is negative and below the threshold, we decay it.
		penalty, err := GeometricDecay(record.Penalty, record.Decay, lastUpdated)
		if err != nil {
			return record, fmt.Errorf("could not decay application specific penalty: %w", err)
		}
		record.Penalty = penalty
		return record, nil
	}
}

// InitAppScoreRecordState initializes the gossipsub spam record state for a peer.
// Returns:
//   - a gossipsub spam record with the default decay value and 0 penalty.
func InitAppScoreRecordState() p2p.GossipSubSpamRecord {
	return p2p.GossipSubSpamRecord{
		Decay:   defaultDecay,
		Penalty: 0,
	}
}
