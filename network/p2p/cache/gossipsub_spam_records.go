package cache

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
	"github.com/onflow/flow-go/network/p2p/logging"
)

// GossipSubSpamRecordCache is a cache for storing the gossipsub spam records of peers. It is thread-safe.
// The spam records of peers is used to calculate the application specific score, which is part of the GossipSub score of a peer.
// Note that neither of the spam records, application specific score, and GossipSub score are shared publicly with other peers.
// Rather they are solely used by the current peer to select the peers to which it will connect on a topic mesh.
type GossipSubSpamRecordCache struct {
	// the in-memory and thread-safe cache for storing the spam records of peers.
	c *stdmap.Backend

	// Optional: the pre-processors to be called upon reading or updating a record in the cache.
	// The pre-processors are called in the order they are added to the cache.
	// The pre-processors are used to perform any necessary pre-processing on the record before returning it.
	// Primary use case is to perform decay operations on the record before reading or updating it. In this way, a
	// record is only decayed when it is read or updated without the need to explicitly iterating over the cache.
	preprocessFns []PreprocessorFunc
}

var _ p2p.GossipSubSpamRecordCache = (*GossipSubSpamRecordCache)(nil)

// PreprocessorFunc is a function that is called by the cache upon reading or updating a record in the cache.
// It is used to perform any necessary pre-processing on the record before returning it when reading or changing it when updating.
// The effect of the pre-processing is that the record is updated in the cache.
// If there are multiple pre-processors, they are called in the order they are added to the cache.
// Args:
//
//	record: the record to be pre-processed.
//	lastUpdated: the last time the record was updated.
//
// Returns:
//
//		GossipSubSpamRecord: the pre-processed record.
//	 error: an error if the pre-processing failed. The error is considered irrecoverable (unless the parameters can be adjusted and the pre-processing can be retried). The caller is
//	 advised to crash the node upon an error if failure to read or update the record is not acceptable.
type PreprocessorFunc func(record p2p.GossipSubSpamRecord, lastUpdated time.Time) (p2p.GossipSubSpamRecord, error)

// NewGossipSubSpamRecordCache returns a new HeroCache-based application specific Penalty cache.
// Args:
//
//	sizeLimit: the maximum number of entries that can be stored in the cache.
//	logger: the logger to be used by the cache.
//	collector: the metrics collector to be used by the cache.
//
// Returns:
//
//	*GossipSubSpamRecordCache: the newly created cache with a HeroCache-based backend.
func NewGossipSubSpamRecordCache(sizeLimit uint32,
	logger zerolog.Logger,
	collector module.HeroCacheMetrics,
	prFns ...PreprocessorFunc) *GossipSubSpamRecordCache {
	backData := herocache.NewCache(sizeLimit,
		herocache.DefaultOversizeFactor,
		// we should not evict any record from the cache,
		// eviction will open the node to spam attacks by malicious peers to erase their application specific penalty.
		heropool.NoEjection,
		logger.With().Str("mempool", "gossipsub-app-Penalty-cache").Logger(),
		collector)
	return &GossipSubSpamRecordCache{
		c:             stdmap.NewBackend(stdmap.WithBackData(backData)),
		preprocessFns: prFns,
	}
}

// Add adds the GossipSubSpamRecord of a peer to the cache.
// Args:
// - peerID: the peer ID of the peer in the GossipSub protocol.
// - record: the GossipSubSpamRecord of the peer.
//
// Returns:
// - bool: true if the record was added successfully, false otherwise.
// Note that a record is added successfully if the cache has enough space to store the record and no record exists for the peer in the cache.
// In other words, the entries are deduplicated by the peer ID.
func (a *GossipSubSpamRecordCache) Add(peerId peer.ID, record p2p.GossipSubSpamRecord) bool {
	entityId := flow.HashToID([]byte(peerId)) // HeroCache uses hash of peer.ID as the unique identifier of the record.
	return a.c.Add(gossipsubSpamRecordEntity{
		entityId:            entityId,
		peerID:              peerId,
		lastUpdated:         time.Now(),
		GossipSubSpamRecord: record,
	})
}

// Update updates the GossipSub spam penalty of a peer in the cache. It assumes that a record already exists for the peer in the cache.
// It first reads the record from the cache, applies the pre-processing functions to the record, and then applies the update function to the record.
// The order of the pre-processing functions is the same as the order in which they were added to the cache.
// Args:
// - peerID: the peer ID of the peer in the GossipSub protocol.
// - updateFn: the update function to be applied to the record.
// Returns:
// - *GossipSubSpamRecord: the updated record.
// - error on failure to update the record. The returned error is irrecoverable and indicates an exception.
// Note that if any of the pre-processing functions returns an error, the record is reverted to its original state (prior to applying the update function).
func (a *GossipSubSpamRecordCache) Update(peerID peer.ID, updateFn p2p.UpdateFunction) (*p2p.GossipSubSpamRecord, error) {
	// HeroCache uses flow.Identifier for keys, so reformat of the peer.ID
	entityId := flow.HashToID([]byte(peerID))
	if !a.c.Has(entityId) {
		return nil, fmt.Errorf("could not update spam records for peer %s, record not found", peerID.String())
	}

	var err error
	record, updated := a.c.Adjust(entityId, func(entry flow.Entity) flow.Entity {
		e := entry.(gossipsubSpamRecordEntity)

		currentRecord := e.GossipSubSpamRecord
		// apply the pre-processing functions to the record.
		for _, apply := range a.preprocessFns {
			e.GossipSubSpamRecord, err = apply(e.GossipSubSpamRecord, e.lastUpdated)
			if err != nil {
				e.GossipSubSpamRecord = currentRecord
				return e // return the original record if the pre-processing fails (atomic abort).
			}
		}

		// apply the update function to the record.
		e.GossipSubSpamRecord = updateFn(e.GossipSubSpamRecord)

		if e.GossipSubSpamRecord != currentRecord {
			e.lastUpdated = time.Now()
		}
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("could not update spam records for peer %s, error: %w", peerID.String(), err)
	}
	if !updated {
		// this happens when the underlying HeroCache fails to update the record.
		return nil, fmt.Errorf("internal cache error for updating %s", peerID.String())
	}

	r := record.(gossipsubSpamRecordEntity).GossipSubSpamRecord
	return &r, nil
}

// Has returns true if the spam record of a peer is found in the cache, false otherwise.
// Args:
// - peerID: the peer ID of the peer in the GossipSub protocol.
// Returns:
// - true if the gossipsub spam record of the peer is found in the cache, false otherwise.
func (a *GossipSubSpamRecordCache) Has(peerID peer.ID) bool {
	entityId := flow.HashToID([]byte(peerID)) // HeroCache uses hash of peer.ID as the unique identifier of the record.
	return a.c.Has(entityId)
}

// Get returns the spam record of a peer from the cache.
// Args:
//
//	-peerID: the peer ID of the peer in the GossipSub protocol.
//
// Returns:
//   - the application specific score record of the peer.
//   - error if the underlying cache update fails, or any of the pre-processors fails. The error is considered irrecoverable, and
//     the caller is advised to crash the node.
//   - true if the record is found in the cache, false otherwise.
func (a *GossipSubSpamRecordCache) Get(peerID peer.ID) (*p2p.GossipSubSpamRecord, error, bool) {
	entityId := flow.HashToID([]byte(peerID)) // HeroCache uses hash of peer.ID as the unique identifier of the record.
	if !a.c.Has(entityId) {
		return nil, nil, false
	}

	var err error
	record, updated := a.c.Adjust(entityId, func(entry flow.Entity) flow.Entity {
		e := entry.(gossipsubSpamRecordEntity)

		currentRecord := e.GossipSubSpamRecord
		for _, apply := range a.preprocessFns {
			e.GossipSubSpamRecord, err = apply(e.GossipSubSpamRecord, e.lastUpdated)
			if err != nil {
				e.GossipSubSpamRecord = currentRecord
				return e // return the original record if the pre-processing fails (atomic abort).
			}
		}
		if e.GossipSubSpamRecord != currentRecord {
			e.lastUpdated = time.Now()
		}
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("error while applying pre-processing functions to cache record for peer %s: %w", logging.PeerId(peerID), err), false
	}
	if !updated {
		return nil, fmt.Errorf("could not decay cache record for peer %s", logging.PeerId(peerID)), false
	}

	r := record.(gossipsubSpamRecordEntity).GossipSubSpamRecord
	return &r, nil, true
}

// GossipSubSpamRecord represents an Entity implementation GossipSubSpamRecord.
// It is internally used by the HeroCache to store the GossipSubSpamRecord.
type gossipsubSpamRecordEntity struct {
	entityId flow.Identifier // the ID of the record (used to identify the record in the cache).
	// lastUpdated is the time at which the record was last updated.
	// the peer ID of the peer in the GossipSub protocol.
	peerID      peer.ID
	lastUpdated time.Time
	p2p.GossipSubSpamRecord
}

// In order to use HeroCache, the gossipsubSpamRecordEntity must implement the flow.Entity interface.
var _ flow.Entity = (*gossipsubSpamRecordEntity)(nil)

// ID returns the ID of the gossipsubSpamRecordEntity. As the ID is used to identify the record in the cache, it must be unique.
// Also, as the ID is used frequently in the cache, it is stored in the record to avoid recomputing it.
// ID is never exposed outside the cache.
func (a gossipsubSpamRecordEntity) ID() flow.Identifier {
	return a.entityId
}

// Checksum returns the same value as ID. Checksum is implemented to satisfy the flow.Entity interface.
// HeroCache does not use the checksum of the gossipsubSpamRecordEntity.
func (a gossipsubSpamRecordEntity) Checksum() flow.Identifier {
	return a.entityId
}
