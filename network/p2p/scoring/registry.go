package scoring

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/p2p"
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
	defaultDecay             = 0.99 // default decay value for the application specific score.
	graftMisbehaviourPenalty = -10
	pruneMisbehaviourPenalty = -10
	iHaveMisbehaviourPenalty = -10
	iWantMisbehaviourPenalty = -10
)

type GossipSubCtrlMsgPenaltyValue struct {
	Graft float64
	Prune float64
	IHave float64
	IWant float64
}

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
	scoreCache *netcache.AppScoreCache
	penalty    GossipSubCtrlMsgPenaltyValue
}

type GossipSubAppSpecificScoreRegistryConfig struct {
	sizeLimit     uint32
	logger        zerolog.Logger
	collector     module.HeroCacheMetrics
	decayFunction netcache.ReadPreprocessorFunc
}

func NewGossipSubAppSpecificScoreRegistry(config GossipSubAppSpecificScoreRegistryConfig) *GossipSubAppSpecificScoreRegistry {
	cache := netcache.NewAppScoreCache(config.sizeLimit, config.logger, config.collector, config.decayFunction)
	return &GossipSubAppSpecificScoreRegistry{
		logger:     config.logger.With().Str("module", "app_score_registry").Logger(),
		scoreCache: cache,
	}
}

var _ p2p.GossipSubInvalidControlMessageNotificationConsumer = (*GossipSubAppSpecificScoreRegistry)(nil)

// AppSpecificScoreFunc returns the application specific score function that is called by the GossipSub protocol to determine the application specific score of a peer.
func (r *GossipSubAppSpecificScoreRegistry) AppSpecificScoreFunc() func(peer.ID) float64 {
	return func(pid peer.ID) float64 {
		record, err, ok := r.scoreCache.Get(pid)
		if err != nil {
			// the error is considered fatal as it means the cache is not working properly.
			// we should not continue with the execution as it may lead to routing attack vulnerability.
			r.logger.Fatal().Str("peer_id", pid.String()).Err(err).Msg("could not get application specific score for peer")
			return 0
		}
		if !ok {
			// this can happen if the peer recently joined the network and its application specific score is not yet cached.
			// nevertheless, we log at the warning level as this should not happen frequently.
			r.logger.Warn().Str("peer_id", pid.String()).Msgf("could not find application specific score for peer")
			return 0
		}

		return record.Score
	}
}

func (r *GossipSubAppSpecificScoreRegistry) OnInvalidControlMessageNotification(notification *p2p.InvalidControlMessageNotification) {
	offendingPeerID := notification.PeerID

	if !r.scoreCache.Has(offendingPeerID) {
		// record does not exist, we create a new one with the default score of 0.
		if err := r.scoreCache.Add(netcache.AppScoreRecord{
			PeerID: offendingPeerID,
			Decay:  defaultDecay,
			Score:  0,
		}); err != nil {
			r.logger.Fatal().
				Str("peer_id", offendingPeerID.String()).
				Err(err).
				Msg("could not add application specific score record to cache")
			return
		}
	}

	lg := r.logger.With().
		Str("peer_id", offendingPeerID.String()).
		Str("misbehavior_type", notification.MsgType.String()).Logger()

	record, err := r.scoreCache.Adjust(offendingPeerID, func(record netcache.AppScoreRecord) netcache.AppScoreRecord {
		switch notification.MsgType {
		case p2p.CtrlMsgGraft:
			record.Score -= r.penalty.Graft
		case p2p.CtrlMsgPrune:
			record.Score -= r.penalty.Prune
		case p2p.CtrlMsgIHave:
			record.Score -= r.penalty.IHave
		case p2p.CtrlMsgIWant:
			record.Score -= r.penalty.IWant
		default:
			lg.Fatal().Str("misbehavior_type", notification.MsgType.String()).Msg("unknown misbehavior type")
		}

		return record
	})

	if err != nil {
		lg.Fatal().Msg("could not apply misbehaviour penalty and update application specific score")
		return
	}

	lg.Debug().
		Float64("app_specific_score", record.Score).
		Msg("applied misbehaviour penalty and updated application specific score")
}

// DefaultDecayFunction is the default decay function that is used to decay the application specific score of a peer.
// It is used if no decay function is provided in the configuration.
// It decays the application specific score of a peer if it is negative.
func DefaultDecayFunction() netcache.ReadPreprocessorFunc {
	return func(record netcache.AppScoreRecord, lastUpdated time.Time) (netcache.AppScoreRecord, error) {
		if record.Score >= 0 {
			// no need to decay the score if it is positive, the reason is currently the app specific score
			// is only used to penalize peers. Hence, when there is no reward, there is no need to decay the positive score, as
			// no node can accumulate a positive score.
			return record, nil
		}

		if record.Score > skipDecayThreshold {
			// score is negative but greater than the threshold, we set it to 0.
			record.Score = 0
			return record, nil
		}

		// score is negative and below the threshold, we decay it.
		score, err := GeometricDecay(record.Score, record.Decay, lastUpdated)
		if err != nil {
			return record, fmt.Errorf("could not decay application specific score for peer %s: %w", record.PeerID, err)
		}
		record.Score = score
		return record, nil
	}
}
