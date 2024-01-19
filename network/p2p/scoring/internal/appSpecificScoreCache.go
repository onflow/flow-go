package internal

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/network/p2p"
)

// AppSpecificScoreCache is a cache that stores the application specific score of peers.
// The application specific score of a peer is used to calculate the GossipSub score of the peer.
// Note that the application specific score and the GossipSub score are solely used by the current peer to select the peers
// to which it will connect on a topic mesh.
type AppSpecificScoreCache struct {
	c *stdmap.Backend
}

var _ p2p.GossipSubApplicationSpecificScoreCache = (*AppSpecificScoreCache)(nil)

// NewAppSpecificScoreCache creates a new application specific score cache with the given size limit.
// The cache has an LRU eviction policy.
// Args:
// - sizeLimit: the size limit of the cache.
// - logger: the logger to use for logging.
// - collector: the metrics collector to use for collecting metrics.
// Returns:
// - *AppSpecificScoreCache: the created cache.
func NewAppSpecificScoreCache(sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics) *AppSpecificScoreCache {
	backData := herocache.NewCache(sizeLimit,
		herocache.DefaultOversizeFactor,
		heropool.LRUEjection,
		logger.With().Str("mempool", "gossipsub-app-specific-score-cache").Logger(),
		collector)

	return &AppSpecificScoreCache{
		c: stdmap.NewBackend(stdmap.WithBackData(backData)),
	}
}

// Get returns the application specific score of a peer from the cache.
// Args:
// - peerID: the peer ID of the peer in the GossipSub protocol.
// Returns:
// - float64: the application specific score of the peer.
// - time.Time: the time at which the score was last updated.
// - bool: true if the score was retrieved successfully, false otherwise.
func (a *AppSpecificScoreCache) Get(peerID peer.ID) (float64, time.Time, bool) {
	e, ok := a.c.ByID(entityIdOf(peerID))
	if !ok {
		return 0, time.Time{}, false
	}
	return e.(appSpecificScoreRecordEntity).Score, e.(appSpecificScoreRecordEntity).LastUpdated, true
}

// AdjustWithInit adds the application specific score of a peer to the cache.
// If the peer already has a score in the cache, the score is updated.
// Args:
// - peerID: the peer ID of the peer in the GossipSub protocol.
// - score: the application specific score of the peer.
// - time: the time at which the score was last updated.
// Returns:
// - error on failure to add the score. The returned error is irrecoverable and indicates an exception.
func (a *AppSpecificScoreCache) AdjustWithInit(peerID peer.ID, score float64, time time.Time) error {
	entityId := entityIdOf(peerID)

	initLogic := func() flow.Entity {
		return appSpecificScoreRecordEntity{
			entityId:    entityId,
			PeerID:      peerID,
			Score:       score,
			LastUpdated: time,
		}
	}
	adjustLogic := func(entity flow.Entity) flow.Entity {
		r := entity.(appSpecificScoreRecordEntity)
		r.Score = score
		r.LastUpdated = time
		return r
	}
	_, adjusted := a.c.AdjustWithInit(entityId, adjustLogic, initLogic)
	if !adjusted {
		return fmt.Errorf("failed to adjust app specific score for peer %s", peerID)
	}

	return nil
}
