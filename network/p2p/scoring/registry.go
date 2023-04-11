package scoring

import (
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// skipDecayThreshold is the threshold for which when the negative score is above this value, the decay function will not be called.
	// instead, the score will be set to 0. This is to prevent the score from keeping a small negative value for a long time.
	skipDecayThreshold = -0.1
	// defaultDecay is the default decay value for the application specific score.
	// this value is used when no custom decay value is provided.
	// this value decays the score by 1% every second.
	// assume that the score is -100 (the maximum application specific score is -100) and the skipDecayThreshold is -0.1,
	// it takes around 459 seconds for the score to decay to reach greater than -0.1 and turn into 0.
	// x * 0.99^n > -0.1 (assuming negative x).
	// 0.99^n > -0.1 / x
	// Now we can take the logarithm of both sides (with any base, but let's use base 10 for simplicity).
	// log(0.99^n) < log(0.1 / x)
	// Using the properties of logarithms, we can bring down the exponent:
	// n * log(0.99) < log(-0.1 / x)
	// And finally, we can solve for n:
	// n > log(-0.1 / x) / log(0.99)
	// We can plug in x = -100:
	// n > log(-0.1 / -100) / log(0.99)
	// n > log(0.001) / log(0.99)
	// n > -3 / log(0.99)
	// n >  458.22
	defaultDecay = 0.99 // default decay value for the application specific score.
	// graftMisbehaviourPenalty is the penalty applied to the application specific score when a peer conducts a graft misbehaviour.
	graftMisbehaviourPenalty = -10
	// pruneMisbehaviourPenalty is the penalty applied to the application specific score when a peer conducts a prune misbehaviour.
	pruneMisbehaviourPenalty = -10
	// iHaveMisbehaviourPenalty is the penalty applied to the application specific score when a peer conducts a iHave misbehaviour.
	iHaveMisbehaviourPenalty = -10
	// iWantMisbehaviourPenalty is the penalty applied to the application specific score when a peer conducts a iWant misbehaviour.
	iWantMisbehaviourPenalty = -10
)

// GossipSubCtrlMsgPenaltyValue is the penalty value for each control message type.
type GossipSubCtrlMsgPenaltyValue struct {
	Graft float64 // penalty value for an individual graft message misbehaviour.
	Prune float64 // penalty value for an individual prune message misbehaviour.
	IHave float64 // penalty value for an individual iHave message misbehaviour.
	IWant float64 // penalty value for an individual iWant message misbehaviour.
}

// DefaultGossipSubCtrlMsgPenaltyValue returns the default penalty value for each control message type.
func DefaultGossipSubCtrlMsgPenaltyValue() GossipSubCtrlMsgPenaltyValue {
	return GossipSubCtrlMsgPenaltyValue{
		Graft: graftMisbehaviourPenalty,
		Prune: pruneMisbehaviourPenalty,
		IHave: iHaveMisbehaviourPenalty,
		IWant: iWantMisbehaviourPenalty,
	}
}

type GossipSubAppSpecificScoreRegistry struct {
	logger     zerolog.Logger
	idProvider module.IdentityProvider
	// spamScoreCache currently only holds the control message misbehaviour score (spam related score).
	spamScoreCache *netcache.GossipSubSpamRecordCache
	penalty        GossipSubCtrlMsgPenaltyValue
	// initial application specific score record, used to initialize the score cache entry.
	init      func() p2p.GossipSubSpamRecord
	validator *SubscriptionValidator
	mu        sync.Mutex
}

type GossipSubAppSpecificScoreRegistryConfig struct {
	SizeLimit     uint32
	Logger        zerolog.Logger
	Validator     *SubscriptionValidator
	Collector     module.HeroCacheMetrics
	DecayFunction netcache.PreprocessorFunc
	Penalty       GossipSubCtrlMsgPenaltyValue
	Init          func() p2p.GossipSubSpamRecord
}

func WithGossipSubAppSpecificScoreRegistryPenalty(penalty GossipSubCtrlMsgPenaltyValue) func(registry *GossipSubAppSpecificScoreRegistry) {
	return func(registry *GossipSubAppSpecificScoreRegistry) {
		registry.penalty = penalty
	}
}

func WithScoreCache(cache *netcache.GossipSubSpamRecordCache) func(registry *GossipSubAppSpecificScoreRegistry) {
	return func(registry *GossipSubAppSpecificScoreRegistry) {
		registry.spamScoreCache = cache
	}
}

func WithRecordInit(init func() p2p.GossipSubSpamRecord) func(registry *GossipSubAppSpecificScoreRegistry) {
	return func(registry *GossipSubAppSpecificScoreRegistry) {
		registry.init = init
	}
}

func NewGossipSubAppSpecificScoreRegistry(config *GossipSubAppSpecificScoreRegistryConfig, opts ...func(registry *GossipSubAppSpecificScoreRegistry)) *GossipSubAppSpecificScoreRegistry {
	cache := netcache.NewGossipSubSpamRecordCache(config.SizeLimit, config.Logger, config.Collector, config.DecayFunction)
	reg := &GossipSubAppSpecificScoreRegistry{
		logger:         config.Logger.With().Str("module", "app_score_registry").Logger(),
		spamScoreCache: cache,
		penalty:        config.Penalty,
		init:           config.Init,
	}

	for _, opt := range opts {
		opt(reg)
	}

	return reg
}

var _ p2p.GossipSubInvalidControlMessageNotificationConsumer = (*GossipSubAppSpecificScoreRegistry)(nil)

// AppSpecificScoreFunc returns the application specific score function that is called by the GossipSub protocol to determine the application specific score of a peer.
func (r *GossipSubAppSpecificScoreRegistry) AppSpecificScoreFunc() func(peer.ID) float64 {
	return func(pid peer.ID) float64 {
		// score of a peer is composed of 3 parts: (1) spam penalty (2) staking score (3) subscription penalty.
		lg := r.logger.With().Str("peer_id", pid.String()).Logger()
		// (1) spam penalty: the penalty is applied to the application specific score when a peer conducts a spamming misbehaviour.
		spamRecord, err, ok := r.spamScoreCache.Get(pid)
		if err != nil {
			// the error is considered fatal as it means the cache is not working properly.
			// we should not continue with the execution as it may lead to routing attack vulnerability.
			r.logger.Fatal().Str("peer_id", pid.String()).Err(err).Msg("could not get application specific score for peer")
			return 0 // unreachable, but added to avoid proceeding with the execution if log level is changed.
		}
		if !ok {
			init := r.init()
			initialized := r.spamScoreCache.Add(pid, init)
			r.logger.Trace().
				Bool("initialized", initialized).
				Str("peer_id", pid.String()).
				Msg("initialization attempt for application specific")
			return init.Penalty
		}

		// (2) staking score: the staking score is the score of a peer based on its role.
		// staking score is applied only if the peer is a staked node and does not have a negative penalty on spamming.
		// it is meant to reward well-behaved staked nodes.
		stakingScore, flowId, role := r.stakingScore(pid)
		if stakingScore > 0 && spamRecord.Penalty < 0 {
			// if the peer is a staked node but has a negative penalty on spamming, we do not apply the
			// staking score and only apply the penalty.
			return spamRecord.Penalty
		}

		// (3) subscription penalty: the subscription penalty is applied to the application specific score when a
		// peer is subscribed to a topic that it is not allowed to subscribe to based on its role.
		subscriptionPenalty := r.subscriptionPenalty(pid, flowId, role)
		appSpecificScore := stakingScore + subscriptionPenalty + spamRecord.Penalty
		lg.Trace().
			Float64("subscription_penalty", subscriptionPenalty).
			Float64("staking_score", stakingScore).
			Float64("spam_penalty", spamRecord.Penalty).
			Float64("total_app_specific_score", appSpecificScore).
			Msg("subscription penalty applied")
		return stakingScore + subscriptionPenalty + spamRecord.Penalty
	}
}

func (r *GossipSubAppSpecificScoreRegistry) stakingScore(pid peer.ID) (float64, flow.Identifier, flow.Role) {
	lg := r.logger.With().Str("peer_id", pid.String()).Logger()

	// checks if peer has a valid Flow protocol identity.
	flowId, err := HasValidFlowIdentity(r.idProvider, pid)
	if err != nil {
		lg.Error().
			Err(err).
			Bool(logging.KeySuspicious, true).
			Msg("invalid peer identity, penalizing peer")
		return MaxAppSpecificPenalty, flow.Identifier{}, 0
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

	return MaxAppSpecificReward, flowId.NodeID, flowId.Role
}

func (r *GossipSubAppSpecificScoreRegistry) subscriptionPenalty(pid peer.ID, flowId flow.Identifier, role flow.Role) float64 {
	// checks if peer has any subscription violation.
	if err := r.validator.CheckSubscribedToAllowedTopics(pid, role); err != nil {
		r.logger.Err(err).
			Str("peer_id", pid.String()).
			Hex("flow_id", logging.ID(flowId)).
			Bool(logging.KeySuspicious, true).
			Msg("invalid subscription detected, penalizing peer")
		return MaxAppSpecificPenalty
	}

	return 0
}

func (r *GossipSubAppSpecificScoreRegistry) OnInvalidControlMessageNotification(notification *p2p.InvalidControlMessageNotification) {
	r.mu.Lock()
	defer r.mu.Unlock()

	lg := r.logger.With().
		Str("peer_id", notification.PeerID.String()).
		Str("misbehavior_type", notification.MsgType.String()).Logger()

	// try initializing the application specific score for the peer if it is not yet initialized.
	// this is done to avoid the case where the peer is not yet cached and the application specific score is not yet initialized.
	// initialization is gone successful only if the peer is not yet cached.
	initialized := r.spamScoreCache.Add(notification.PeerID, r.init())
	lg.Trace().Bool("initialized", initialized).Msg("initialization attempt for application specific")

	record, err := r.spamScoreCache.Adjust(notification.PeerID, func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		switch notification.MsgType {
		case p2p.CtrlMsgGraft:
			record.Penalty += r.penalty.Graft
		case p2p.CtrlMsgPrune:
			record.Penalty += r.penalty.Prune
		case p2p.CtrlMsgIHave:
			record.Penalty += r.penalty.IHave
		case p2p.CtrlMsgIWant:
			record.Penalty += r.penalty.IWant
		default:
			// the error is considered fatal as it means that we have an unsupported misbehaviour type, we should crash the node to prevent routing attack vulnerability.
			lg.Fatal().Str("misbehavior_type", notification.MsgType.String()).Msg("unknown misbehaviour type")
		}

		return record
	})

	if err != nil {
		// any returned error from adjust is non-recoverable and fatal, we crash the node.
		lg.Fatal().Err(err).Msg("could not adjust application specific score for peer")
	}

	lg.Debug().
		Float64("app_specific_score", record.Penalty).
		Msg("applied misbehaviour penalty and updated application specific score")
}

// DefaultDecayFunction is the default decay function that is used to decay the application specific score of a peer.
// It is used if no decay function is provided in the configuration.
// It decays the application specific score of a peer if it is negative.
func DefaultDecayFunction() netcache.PreprocessorFunc {
	return func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error) {
		if record.Penalty >= 0 {
			// no need to decay the score if it is positive, the reason is currently the app specific score
			// is only used to penalize peers. Hence, when there is no reward, there is no need to decay the positive score, as
			// no node can accumulate a positive score.
			return record, nil
		}

		if record.Penalty > skipDecayThreshold {
			// score is negative but greater than the threshold, we set it to 0.
			record.Penalty = 0
			return record, nil
		}

		// score is negative and below the threshold, we decay it.
		score, err := GeometricDecay(record.Penalty, record.Decay, lastUpdated)
		if err != nil {
			return record, fmt.Errorf("could not decay application specific score: %w", err)
		}
		record.Penalty = score
		return record, nil
	}
}

func InitAppScoreRecordState() p2p.GossipSubSpamRecord {
	return p2p.GossipSubSpamRecord{
		Decay:   defaultDecay,
		Penalty: 0,
	}
}
