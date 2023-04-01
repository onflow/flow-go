package netcache

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
)

// AppScoreCache is a cache for storing the application specific Score of a peer in the GossipSub protocol.
// AppSpecificScore is a function that is called by the GossipSub protocol to determine the application specific Score of a peer.
// The application specific Score part of the GossipSub Score a peer and contributes to the overall Score that
// selects the peers to which the current peer will connect on a topic mesh.
// Note that neither the GossipSub Score nor its application specific Score part are shared with the other peers.
// Rather it is solely used by the current peer to select the peers to which it will connect on a topic mesh.
type AppScoreCache struct {
	c             *stdmap.Backend
	preprocessFns []ReadPreprocessorFunc
}

// ReadPreprocessorFunc is a function that is called by the cache upon reading an entry from the cache and before returning it.
// It is used to perform any necessary pre-processing on the entry before returning it.
// The effect of the pre-processing is that the entry is updated in the cache.
// If there are multiple pre-processors, they are called in the order they are added to the cache.
// Args:
//
//	record: the entry to be pre-processed.
//	lastUpdated: the last time the entry was updated.
//
// Returns:
//
//	AppScoreRecord: the pre-processed entry.
type ReadPreprocessorFunc func(record AppScoreRecord, lastUpdated time.Time) (AppScoreRecord, error)

// NewAppScoreCache returns a new HeroCache-based application specific Score cache.
// Args:
//
//	sizeLimit: the maximum number of entries that can be stored in the cache.
//	logger: the logger to be used by the cache.
//	collector: the metrics collector to be used by the cache.
//
// Returns:
//
//	*AppScoreCache: the newly created cache with a HeroCache-based backend.
func NewAppScoreCache(sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics, prFns ...ReadPreprocessorFunc) *AppScoreCache {
	backData := herocache.NewCache(sizeLimit,
		herocache.DefaultOversizeFactor,
		// we should not evict any entry from the cache,
		// as it is used to store the application specific Score of a peer,
		// so ejection is disabled to avoid throwing away the app specific Score of a peer.
		heropool.NoEjection,
		logger.With().Str("mempool", "gossipsub-app-Score-cache").Logger(),
		collector)
	return &AppScoreCache{
		c:             stdmap.NewBackend(stdmap.WithBackData(backData)),
		preprocessFns: prFns,
	}
}

// Add adds the application specific Score of a peer to the cache if not already present, or
// updates the application specific Score of a peer in the cache if already present.
// Args:
//
//		PeerID: the peer ID of the peer in the GossipSub protocol.
//		Decay: the Decay factor of the application specific Score of the peer. Must be in the range [0, 1].
//	    Score: the application specific Score of the peer.
//
// Returns:
//
//		error on illegal argument (e.g., invalid Decay) or if the application specific Score of the peer
//	    could not be added or updated. The returned error  is irrecoverable and the caller should crash the node.
//	    The returned error means either the cache is full or the cache is in an inconsistent state.
//	    Either case, the caller should crash the node to avoid inconsistent state.
//		If the update fails, the application specific Score of the peer will not be used
//		and this makes the GossipSub protocol vulnerable if the peer is malicious. As when there is no record of
//		the application specific Score of a peer, the GossipSub considers the peer to have a Score of 0, and
//		this does not prevent the GossipSub protocol from connecting to the peer on a topic mesh.
func (a *AppScoreCache) Add(record AppScoreRecord) error {
	entityId := flow.HashToID([]byte(record.PeerID)) // HeroCache uses hash of peer.ID as the unique identifier of the entry.
	switch exists := a.c.Has(entityId); {
	case exists:
		_, updated := a.c.Adjust(entityId, func(entry flow.Entity) flow.Entity {
			appScoreCacheEntry := entry.(appScoreRecordEntity)
			appScoreCacheEntry.AppScoreRecord = record
			return appScoreCacheEntry
		})
		if !updated {
			return fmt.Errorf("could not update app Score cache entry for peer %s", record.PeerID)
		}
	case !exists:
		if added := a.c.Add(appScoreRecordEntity{
			entityId:       entityId,
			AppScoreRecord: record,
		}); !added {
			return fmt.Errorf("could not add app Score cache entry for peer %s", record.PeerID)
		}
	}

	return nil
}

// Adjust adjusts the application specific Score of a peer in the cache.
// It first reads the entry from the cache, applies the update function to the entry, and then runs the pre-processing functions on the entry.
// The order of the pre-processing functions is the same as the order in which they were added to the cache.
// Args:
// - peerID: the peer ID of the peer in the GossipSub protocol.
// - updateFn: the update function to be applied to the entry.
// Returns:
// - *AppScoreRecord: the updated entry.
// - error on failure to update the entry. The returned error is irrecoverable and the caller should crash the node.
// Note that if any of the pre-processing functions returns an error, the entry is reverted to its original state (prior to applying the update function).
func (a *AppScoreCache) Adjust(peerID peer.ID, updateFn func(record AppScoreRecord) AppScoreRecord) (*AppScoreRecord, error) {
	entityId := flow.HashToID([]byte(peerID)) // HeroCache uses hash of peer.ID as the unique identifier of the entry.
	if !a.c.Has(entityId) {
		return nil, fmt.Errorf("could not adjust app Score cache entry for peer %s, entry not found", peerID)
	}

	var err error
	record, updated := a.c.Adjust(entityId, func(entry flow.Entity) flow.Entity {
		e := entry.(appScoreRecordEntity)

		currentRecord := e.AppScoreRecord

		// apply the update function to the entry.
		e.AppScoreRecord = updateFn(e.AppScoreRecord)

		// apply the pre-processing functions to the entry.
		for _, apply := range a.preprocessFns {
			e.AppScoreRecord, err = apply(e.AppScoreRecord, e.lastUpdated)
			if err != nil {
				e.AppScoreRecord = currentRecord
				return e // return the original entry if the pre-processing fails (atomic abort).
			}
		}
		if e.AppScoreRecord != currentRecord {
			e.lastUpdated = time.Now()
		}
		return e
	})
	if !updated {
		return nil, fmt.Errorf("could not decay cache entry for peer %s", peerID)
	}

	r := record.(appScoreRecordEntity).AppScoreRecord
	return &r, nil
}

// Get returns the application specific Score of a peer from the cache.
// Args:
//
//	PeerID: the peer ID of the peer in the GossipSub protocol.
//
// Returns:
//   - the application specific score record of the peer.
//   - error if the underlying HeroCache update fails, or any of the pre-processors fails. The error is considered irrecoverable, and
//     the caller should crash the node.
//   - true if the application specific Score of the peer is found in the cache, false otherwise.
func (a *AppScoreCache) Get(peerID peer.ID) (*AppScoreRecord, error, bool) {
	entityId := flow.HashToID([]byte(peerID)) // HeroCache uses hash of peer.ID as the unique identifier of the entry.
	if !a.c.Has(entityId) {
		return nil, nil, false
	}

	var err error
	record, updated := a.c.Adjust(entityId, func(entry flow.Entity) flow.Entity {
		e := entry.(appScoreRecordEntity)

		currentRecord := e.AppScoreRecord
		for _, apply := range a.preprocessFns {
			e.AppScoreRecord, err = apply(e.AppScoreRecord, e.lastUpdated)
			if err != nil {
				e.AppScoreRecord = currentRecord
				return e // return the original entry if the pre-processing fails (atomic abort).
			}
		}
		if e.AppScoreRecord != currentRecord {
			e.lastUpdated = time.Now()
		}
		return e
	})
	if !updated {
		return nil, fmt.Errorf("could not decay cache entry for peer %s", peerID), false
	}

	r := record.(appScoreRecordEntity).AppScoreRecord
	return &r, nil, true
}

// AppScoreRecord represents the application specific Score of a peer in the GossipSub protocol.
// It acts as a Score card for a peer in the GossipSub protocol that keeps the
// application specific Score of the peer and its Decay factor.
type AppScoreRecord struct {
	entityId flow.Identifier // the ID of the entry (used to identify the entry in the cache).

	// the peer ID of the peer in the GossipSub protocol.
	PeerID peer.ID

	// Decay factor of the app specific Score.
	// the app specific Score is multiplied by the Decay factor every time the Score is updated if the Score is negative.
	// this is to prevent the Score from being stuck at a negative value.
	// each peer has its own Decay factor based on its behavior.
	// value is in the range [0, 1].
	Decay float64
	// Score is the application specific Score of the peer.
	Score float64
}

// AppScoreRecord represents an entry for the AppScoreCache.
// It stores the application specific Score of a peer in the GossipSub protocol.
type appScoreRecordEntity struct {
	entityId flow.Identifier // the ID of the entry (used to identify the entry in the cache).
	// lastUpdated is the time at which the entry was last updated.
	lastUpdated time.Time
	AppScoreRecord
}

// In order to use HeroCache, the entry must implement the flow.Entity interface.
var _ flow.Entity = (*appScoreRecordEntity)(nil)

// ID returns the ID of the entry. As the ID is used to identify the entry in the cache, it must be unique.
// Also, as the ID is used frequently in the cache, it is stored in the entry to avoid recomputing it.
// ID is never exposed outside the cache.
func (a appScoreRecordEntity) ID() flow.Identifier {
	return a.entityId
}

// Checksum returns the same value as ID. Checksum is implemented to satisfy the flow.Entity interface.
// HeroCache does not use the checksum of the entry.
func (a appScoreRecordEntity) Checksum() flow.Identifier {
	return a.entityId
}
