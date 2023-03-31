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
)

type GossipSubAppSpecificScoreRegistry struct {
	logger     zerolog.Logger
	scoreCache *netcache.AppScoreCache
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
		//record, ok := r.scoreCache.Get(pid)
		//if !ok {
		//	// this can happen if the peer recently joined the network and its application specific score is not yet cached.
		//	// nevertheless, we log at the warning level as this should not happen frequently.
		//	r.logger.Warn().Str("peer_id", pid.String()).Msgf("could not find application specific score for peer")
		//	return 0
		//}
		//
		//if record.Score < 0 {
		//	record.Score = GeometricDecay(record.Score, record.Decay, record.)
		//	r.logger.Trace().Str("peer_id", pid.String()).Float64("score", record.Score).Msgf("decayed application specific score for peer")
		//}
		//
		//record.LastUpdated = time.Now()
		//err := r.scoreCache.Update(*record)
		//if err != nil {
		//	r.logger.Fatal().Err(err).Str("peer_id", pid.String()).Msgf("could not update application specific score for peer")
		//}
		//
		//return record.Score
		return 0
	}
}

func (r *GossipSubAppSpecificScoreRegistry) OnInvalidControlMessageNotification(notification *p2p.InvalidControlMessageNotification) {

}

// DefaultDecayFunction is the default decay function that is used to decay the application specific score of a peer.
// It is used if no decay function is provided in the configuration.
// It decays the application specific score of a peer if it is negative.
func DefaultDecayFunction(record *netcache.AppScoreRecord) netcache.ReadPreprocessorFunc {
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
