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
	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
	"github.com/onflow/flow-go/network/p2p/scoring"
)

const (
	// skipDecayThreshold is the threshold for which when the counter is below this value, the decay function will not be called.
	// instead, the counter will be set to 0. This is to prevent the counter from becoming a large number over time.
	skipDecayThreshold = 0.1
)

// GossipSubDuplicateMessageTrackerCache is a cache used to store the current count of duplicate messages detected
// from a peer. This count is utilized to calculate a penalty for duplicate messages, which is then applied
// to the peer's application-specific score. The duplicate message tracker decays over time to prevent perpetual
// penalization of a peer.
type GossipSubDuplicateMessageTrackerCache struct {
	// the in-memory and thread-safe cache for storing the spam records of peers.
	c *stdmap.Backend

	// recordDecay the recordDecay for the counters stored in the cache. This recordDecay is applied to the counter before it is returned from the cache.
	recordDecay float64
}

// NewGossipSubDuplicateMessageTrackerCache returns a new HeroCache-based duplicate message counter cache.
// Args:
//
//	sizeLimit: the maximum number of entries that can be stored in the cache.
//	decay: the record decay.
//	logger: the logger to be used by the cache.
//	collector: the metrics collector to be used by the cache.
//
// Returns:
//   - *GossipSubDuplicateMessageTrackerCache: the newly created cache with a HeroCache-based backend.
func NewGossipSubDuplicateMessageTrackerCache(sizeLimit uint32, decay float64, logger zerolog.Logger, collector module.HeroCacheMetrics) *GossipSubDuplicateMessageTrackerCache {
	backData := herocache.NewCache(sizeLimit,
		herocache.DefaultOversizeFactor,
		// we should not evict any record from the cache,
		// eviction will open the node to spam attacks by malicious peers to erase their duplicate message counters reducing the overall penalty applied application specific score.
		heropool.NoEjection,
		logger.With().Str("mempool", "gossipsub-duplicate-message-counter-cache").Logger(),
		collector)
	return &GossipSubDuplicateMessageTrackerCache{
		recordDecay: decay,
		c:           stdmap.NewBackend(stdmap.WithBackData(backData)),
	}
}

func (g *GossipSubDuplicateMessageTrackerCache) init(peerID peer.ID) bool {
	entityID := flow.HashToID([]byte(peerID)) // HeroCache uses hash of peer.ID as the unique identifier of the record.
	return g.c.Add(NewDuplicateMessagesCounter(entityID))
}

// Get returns the current number of duplicate messages encountered from a peer. The counter is decayed before being
// returned.
// Args:
// - peerID: the peer ID of the peer in the GossipSub protocol.
//
// Returns:
// - float64: updated value for the duplicate message tracker.
// - error: if any error was encountered during record adjustments in cache.
//   - true if the record is found in the cache, false otherwise.
//
// No errors are expected during normal operation.
func (g *GossipSubDuplicateMessageTrackerCache) Get(peerID peer.ID) (float64, error, bool) {
	entityID := flow.HashToID([]byte(peerID)) // HeroCache uses hash of peer.ID as the unique identifier of the record.
	if !g.c.Has(entityID) {
		return 0, nil, false
	}

	var err error
	adjustedEntity, updated := g.c.Adjust(entityID, func(entity flow.Entity) flow.Entity {
		// perform recordDecay on gauge value
		record := duplicateMessagesCounter(entity)
		entity, err = g.decayAdjustment(record)
		return entity
	})
	if err != nil {
		return 0, fmt.Errorf("error while applying pre-processing functions to cache record for peer %s: %w", p2plogging.PeerId(peerID), err), false
	}
	if !updated {
		return 0, fmt.Errorf("could not decay cache record for peer %s", p2plogging.PeerId(peerID)), false
	}
	record := duplicateMessagesCounter(adjustedEntity)
	return record.Gauge, nil, true
}

// Inc increments the number of duplicate messages detected for the peer. This func is used in conjunction with the GossipSubMeshTracer and is invoked
// each time the DuplicateMessage callback is invoked.
// Args:
// - peerID: the peer ID of the peer in the GossipSub protocol.
//
// Returns:
// - float64: updated value for the duplicate message tracker.
// - error: if any error was encountered during record adjustments in cache.
// No errors are expected during normal operation.
func (g *GossipSubDuplicateMessageTrackerCache) Inc(peerID peer.ID) (float64, error) {
	var err error
	entityID := flow.HashToID([]byte(peerID))
	optimisticAdjustFunc := func() (flow.Entity, bool) {
		return g.c.Adjust(entityID, func(entity flow.Entity) flow.Entity {
			record := duplicateMessagesCounter(entity)
			entity, err = g.decayAdjustment(record) // first recordDecay the record
			if err != nil {
				return entity
			}
			return g.incrementAdjustment(entity) // then increment the record
		})
	}

	// optimisticAdjustFunc is called assuming the record exists; if the record does not exist,
	// it means the record was not initialized. In this case, initialize the record and call optimisticAdjustFunc again.
	// If the record was initialized, optimisticAdjustFunc will be called only once.
	adjustedEntity, adjusted := optimisticAdjustFunc()
	if err != nil {
		return 0, fmt.Errorf("unexpected error while applying recordDecay adjustment for peerID %s: %w", peerID, err)
	}
	if !adjusted {
		g.init(peerID)
		adjustedEntity, adjusted = optimisticAdjustFunc()
		if !adjusted {
			return 0, fmt.Errorf("unexpected record not found for peerID %s, even after an init attempt", peerID)
		}
	}

	return adjustedEntity.(DuplicateMessagesCounterEntity).Gauge, nil
}

// incrementAdjustment performs a cache adjustment that increments the guage for the DuplicateMessagesCounterEntity
func (g *GossipSubDuplicateMessageTrackerCache) incrementAdjustment(entity flow.Entity) flow.Entity {
	record := duplicateMessagesCounter(entity)
	record.Gauge++
	record.lastUpdated = time.Now()
	// Return the adjusted record.
	return record
}

// decayAdjustment performs geometric recordDecay on the duplicate message counter gauge of a peer. This ensures a peer is not penalized forever.
func (g *GossipSubDuplicateMessageTrackerCache) decayAdjustment(counter DuplicateMessagesCounterEntity) (DuplicateMessagesCounterEntity, error) {
	received := counter.Gauge
	if received == 0 {
		return counter, nil
	}

	if received < skipDecayThreshold {
		counter.Gauge = 0
		return counter, nil
	}

	decayedVal, err := scoring.GeometricDecay(received, g.recordDecay, counter.lastUpdated)
	if err != nil {
		return counter, fmt.Errorf("could not recordDecay duplicate message counter gauge: %w", err)
	}
	counter.Gauge = decayedVal
	return counter, nil
}
