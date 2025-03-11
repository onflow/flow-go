package cache

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/network/p2p"
	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
)

// GossipSubSpamRecordCache is a cache for storing the gossipsub spam records of peers. It is thread-safe.
// The spam records of peers is used to calculate the application specific score, which is part of the GossipSub score of a peer.
// Note that neither of the spam records, application specific score, and GossipSub score are shared publicly with other peers.
// Rather they are solely used by the current peer to select the peers to which it will connect on a topic mesh.
type GossipSubSpamRecordCache struct {
	// the in-memory and thread-safe cache for storing the spam records of peers.
	c *stdmap.Backend[peer.ID, *gossipSubSpamRecordWrapper]

	// Optional: the pre-processors to be called upon reading or updating a record in the cache.
	// The pre-processors are called in the order they are added to the cache.
	// The pre-processors are used to perform any necessary pre-processing on the record before returning it.
	// Primary use case is to perform decay operations on the record before reading or updating it. In this way, a
	// record is only decayed when it is read or updated without the need to explicitly iterating over the cache.
	preprocessFns []PreprocessorFunc

	// initFn is a function that is called to initialize a new record in the cache.
	initFn func() p2p.GossipSubSpamRecord
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
	initFn func() p2p.GossipSubSpamRecord,
	prFns ...PreprocessorFunc) *GossipSubSpamRecordCache {
	backData := herocache.NewCache[peer.ID, *gossipSubSpamRecordWrapper](sizeLimit,
		herocache.DefaultOversizeFactor,
		heropool.LRUEjection,
		logger.With().Str("mempool", "gossipsub-app-Penalty-cache").Logger(),
		collector)
	return &GossipSubSpamRecordCache{
		c:             stdmap.NewBackend(stdmap.WithMutableBackData[peer.ID, *gossipSubSpamRecordWrapper](backData)),
		preprocessFns: prFns,
		initFn:        initFn,
	}
}

// Adjust updates the GossipSub spam penalty of a peer in the cache. If the peer does not have a record in the cache, a new record is created.
// The order of the pre-processing functions is the same as the order in which they were added to the cache.
// Args:
// - peerID: the peer ID of the peer in the GossipSub protocol.
// - updateFn: the update function to be applied to the record.
// Returns:
// - *GossipSubSpamRecord: the updated record.
// - error on failure to update the record. The returned error is irrecoverable and indicates an exception.
// Note that if any of the pre-processing functions returns an error, the record is reverted to its original state (prior to applying the update function).
func (a *GossipSubSpamRecordCache) Adjust(peerID peer.ID, updateFn p2p.UpdateFunction) (*p2p.GossipSubSpamRecord, error) {
	var err error
	adjustFunc := func(gossipSubSpamRecordWrapper *gossipSubSpamRecordWrapper) *gossipSubSpamRecordWrapper {
		currentRecord := gossipSubSpamRecordWrapper.GossipSubSpamRecord
		// apply the pre-processing functions to the record.
		for _, apply := range a.preprocessFns {
			gossipSubSpamRecordWrapper.GossipSubSpamRecord, err = apply(gossipSubSpamRecordWrapper.GossipSubSpamRecord, gossipSubSpamRecordWrapper.lastUpdated)
			if err != nil {
				gossipSubSpamRecordWrapper.GossipSubSpamRecord = currentRecord
				return gossipSubSpamRecordWrapper // return the original record if the pre-processing fails (atomic abort).
			}
		}

		// apply the update function to the record.
		gossipSubSpamRecordWrapper.GossipSubSpamRecord = updateFn(gossipSubSpamRecordWrapper.GossipSubSpamRecord)

		if gossipSubSpamRecordWrapper.GossipSubSpamRecord != currentRecord {
			gossipSubSpamRecordWrapper.lastUpdated = time.Now()
		}
		return gossipSubSpamRecordWrapper
	}

	initFunc := func() *gossipSubSpamRecordWrapper {
		return &gossipSubSpamRecordWrapper{
			GossipSubSpamRecord: a.initFn(),
		}
	}

	adjustedWrapper, adjusted := a.c.AdjustWithInit(peerID, adjustFunc, initFunc)
	if err != nil {
		return nil, fmt.Errorf("error while applying pre-processing functions to cache record for peer %s: %w", p2plogging.PeerId(peerID), err)
	}
	if !adjusted {
		return nil, fmt.Errorf("could not adjust cache record for peer %s", p2plogging.PeerId(peerID))
	}

	return &adjustedWrapper.GossipSubSpamRecord, nil
}

// Has returns true if the spam record of a peer is found in the cache, false otherwise.
// Args:
// - peerID: the peer ID of the peer in the GossipSub protocol.
// Returns:
// - true if the gossipsub spam record of the peer is found in the cache, false otherwise.
func (a *GossipSubSpamRecordCache) Has(peerID peer.ID) bool {
	return a.c.Has(peerID)
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
	if !a.c.Has(peerID) {
		return nil, nil, false
	}

	var err error
	record, updated := a.c.Adjust(peerID, func(gossipSubSpamRecordWrapper *gossipSubSpamRecordWrapper) *gossipSubSpamRecordWrapper {
		currentRecord := gossipSubSpamRecordWrapper.GossipSubSpamRecord
		for _, apply := range a.preprocessFns {
			gossipSubSpamRecordWrapper.GossipSubSpamRecord, err = apply(gossipSubSpamRecordWrapper.GossipSubSpamRecord, gossipSubSpamRecordWrapper.lastUpdated)
			if err != nil {
				gossipSubSpamRecordWrapper.GossipSubSpamRecord = currentRecord
				return gossipSubSpamRecordWrapper // return the original record if the pre-processing fails (atomic abort).
			}
		}
		if gossipSubSpamRecordWrapper.GossipSubSpamRecord != currentRecord {
			gossipSubSpamRecordWrapper.lastUpdated = time.Now()
		}
		return gossipSubSpamRecordWrapper
	})
	if err != nil {
		return nil, fmt.Errorf("error while applying pre-processing functions to cache record for peer %s: %w", p2plogging.PeerId(peerID), err), false
	}
	if !updated {
		return nil, fmt.Errorf("could not decay cache record for peer %s", p2plogging.PeerId(peerID)), false
	}

	return &record.GossipSubSpamRecord, nil, true
}

// gossipSubSpamRecordWrapper represents a wrapper around the GossipSubSpamRecord.
// It is internally used by the HeroCache to store the GossipSubSpamRecord.
type gossipSubSpamRecordWrapper struct {
	// lastUpdated is the time at which the record was last updated.
	lastUpdated time.Time
	p2p.GossipSubSpamRecord
}
