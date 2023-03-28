package netcache

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

// AppScoreCache is a cache for storing the application specific score of a peer in the GossipSub protocol.
// AppSpecificScore is a function that is called by the GossipSub protocol to determine the application specific score of a peer.
// The application specific score part of the GossipSub score a peer and contributes to the overall score that
// selects the peers to which the current peer will connect on a topic mesh.
// Note that neither the GossipSub score nor its application specific score part are shared with the other peers.
// Rather it is solely used by the current peer to select the peers to which it will connect on a topic mesh.
type AppScoreCache struct {
	c *stdmap.Backend
}

// NewAppScoreCache returns a new HeroCache-based application specific score cache.
// Args:
//
//	sizeLimit: the maximum number of entries that can be stored in the cache.
//	logger: the logger to be used by the cache.
//	collector: the metrics collector to be used by the cache.
//
// Returns:
//
//	*AppScoreCache: the newly created cache with a HeroCache-based backend.
func NewAppScoreCache(sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics) *AppScoreCache {
	backData := herocache.NewCache(sizeLimit,
		herocache.DefaultOversizeFactor,
		// we should not evict any entry from the cache,
		// as it is used to store the application specific score of a peer,
		// so ejection is disabled to avoid throwing away the app specific score of a peer.
		heropool.NoEjection,
		logger.With().Str("mempool", "gossipsub-app-score-cache").Logger(),
		collector)
	return &AppScoreCache{
		c: stdmap.NewBackend(stdmap.WithBackData(backData)),
	}
}

// Update adds the application specific score of a peer to the cache if not already present, or
// updates the application specific score of a peer in the cache if already present.
// Args:
//
//		peerID: the peer ID of the peer in the GossipSub protocol.
//		decay: the decay factor of the application specific score of the peer.
//	 score: the application specific score of the peer.
//
// Returns:
//
//		error if the application specific score of the peer could not be added or updated. The returned error
//	 is irrecoverable and the caller should crash the node. The returned error means either the cache is full
//	 or the cache is in an inconsistent state. Either case, the caller should crash the node to avoid
//	 inconsistent state. If the update fails, the application specific score of the peer will not be used
//	 and this makes the GossipSub protocol vulnerable if the peer is malicious. As when there is no record of
//	 the application specific score of a peer, the GossipSub considers the peer to have a score of 0, and
//	 this does not prevent the GossipSub protocol from connecting to the peer on a topic mesh.
func (a *AppScoreCache) Update(peerID peer.ID, decay float64, score float64) error {
	entityId := flow.HashToID([]byte(peerID)) // HeroCache uses hash of peer.ID as the unique identifier of the entry.
	switch exists := a.c.Has(entityId); {
	case exists:
		_, updated := a.c.Adjust(entityId, func(entry flow.Entity) flow.Entity {
			appScoreCacheEntry := entry.(appScoreCacheEntry)
			appScoreCacheEntry.decay = decay
			appScoreCacheEntry.score = score
			return appScoreCacheEntry
		})
		if !updated {
			return fmt.Errorf("could not update app score cache entry for peer %s", peerID)
		}
	case !exists:
		if added := a.c.Add(appScoreCacheEntry{
			entityId: entityId,
			peerID:   peerID,
			decay:    decay,
			score:    score,
		}); !added {
			return fmt.Errorf("could not add app score cache entry for peer %s", peerID)
		}
	}

	return nil
}

// Get returns the application specific score of a peer from the cache.
// Args:
//
//	peerID: the peer ID of the peer in the GossipSub protocol.
//
// Returns:
//   - the application specific score of the peer.
//   - the decay factor of the application specific score of the peer.
//   - true if the application specific score of the peer is found in the cache, false otherwise.
func (a *AppScoreCache) Get(peerID peer.ID) (float64, float64, bool) {
	entityId := flow.HashToID([]byte(peerID)) // HeroCache uses hash of peer.ID as the unique identifier of the entry.
	entry, exists := a.c.ByID(entityId)
	if !exists {
		return 0, 0, false
	}
	appScoreCacheEntry := entry.(appScoreCacheEntry)
	return appScoreCacheEntry.score, appScoreCacheEntry.decay, true
}

// appScoreCacheEntry represents an entry for the AppScoreCache
// It stores the application specific score of a peer in the GossipSub protocol.
type appScoreCacheEntry struct {
	entityId flow.Identifier // the ID of the entry (used to identify the entry in the cache).

	peerID peer.ID // the peer ID of the peer in the GossipSub protocol.
	// the decay factor of the app specific score.
	// the app specific score is multiplied by the decay factor every time the score is updated if the score is negative.
	// this is to prevent the score from being stuck at a negative value.
	// each peer has its own decay factor based on its behavior.
	// value is in the range [0, 1].
	decay float64
	// the application specific score of the peer.
	score float64
}

// ID returns the ID of the entry. As the ID is used to identify the entry in the cache, it must be unique.
// Also, as the ID is used frequently in the cache, it is stored in the entry to avoid recomputing it.
// ID is never exposed outside the cache.
func (a appScoreCacheEntry) ID() flow.Identifier {
	return a.entityId
}

// Checksum returns the same value as ID. Checksum is implemented to satisfy the flow.Entity interface.
// HeroCache does not use the checksum of the entry.
func (a appScoreCacheEntry) Checksum() flow.Identifier {
	return a.entityId
}

// In order to use HeroCache, the entry must implement the flow.Entity interface.
var _ flow.Entity = (*appScoreCacheEntry)(nil)
