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
	"github.com/onflow/flow-go/network/p2p/scoring"
)

// GossipSubDuplicateMessageTrackerCache is a cache used to store the current count of duplicate messages detected
// from a peer. This count is utilized to calculate a penalty for duplicate messages, which is then applied
// to the peer's application-specific score. The duplicate message tracker decays over time to prevent perpetual
// penalization of a peer.
type GossipSubDuplicateMessageTrackerCache struct {
	// the in-memory and thread-safe cache for storing the spam records of peers.
	c *stdmap.Backend

	// Optional: the pre-processors to be called upon reading or updating a record in the cache.
	// The pre-processors are called in the order they are added to the cache.
	// The pre-processors are used to perform any necessary pre-processing on the record before returning it.
	// Primary use case is to perform recordDecay operations on the record before reading or updating it. In this way, a
	// record is only decayed when it is read or updated without the need to explicitly iterating over the cache.
	preprocessFns []PreprocessorFunc

	// recordDecay the recordDecay for the counters stored in the cache. This recordDecay is applied to the counter before it is returned from the cache.
	recordDecay float64
}

var _ p2p.GossipSubDuplicateMessageTrackerCache = (*GossipSubDuplicateMessageTrackerCache)(nil)

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
func NewGossipSubDuplicateMessageTrackerCache(sizeLimit uint32, decay float64, logger zerolog.Logger, collector module.HeroCacheMetrics, prFns ...PreprocessorFunc) *GossipSubDuplicateMessageTrackerCache {
	backData := herocache.NewCache(sizeLimit,
		herocache.DefaultOversizeFactor,
		// we should not evict any record from the cache,
		// eviction will open the node to spam attacks by malicious peers to erase their duplicate message counters reducing the overall penalty applied application specific score.
		heropool.NoEjection,
		logger.With().Str("mempool", "gossipsub-duplicate-message-counter-cache").Logger(),
		collector)
	return &GossipSubDuplicateMessageTrackerCache{
		recordDecay:   decay,
		c:             stdmap.NewBackend(stdmap.WithBackData(backData)),
		preprocessFns: prFns,
	}
}

func (g *GossipSubDuplicateMessageTrackerCache) init(peerId peer.ID) bool {
	entityId := flow.HashToID([]byte(peerId)) // HeroCache uses hash of peer.ID as the unique identifier of the record.
	return g.c.Add(NewDuplicateMessagesCounter(entityId))
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
func (g *GossipSubDuplicateMessageTrackerCache) Inc(peerId peer.ID) (float64, error) {
	var err error
	entityID := flow.HashToID([]byte(peerId))
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
		return 0, fmt.Errorf("unexpected error while applying recordDecay adjustment for peerID %s: %w", peerId, err)
	}
	if !adjusted {
		g.init(peerId)
		adjustedEntity, adjusted = optimisticAdjustFunc()
		if !adjusted {
			return 0, fmt.Errorf("unexpected record not found for peerID %s, even after an init attempt", peerId)
		}
	}

	return adjustedEntity.(DuplicateMessagesCounter).Gauge, nil
}

// incrementAdjustment performs a cache adjustment that increments the guage for the DuplicateMessagesCounter
func (g *GossipSubDuplicateMessageTrackerCache) incrementAdjustment(entity flow.Entity) flow.Entity {
	record := duplicateMessagesCounter(entity)
	record.Gauge++
	record.lastUpdated = time.Now()
	// Return the adjusted record.
	return record
}

// decayAdjustment performs geometric recordDecay on the duplicate message counter gauge of a peer. This ensures a peer is not penalized forever.
func (g *GossipSubDuplicateMessageTrackerCache) decayAdjustment(counter DuplicateMessagesCounter) (DuplicateMessagesCounter, error) {
	received := counter.Gauge
	if received == 0 {
		return counter, nil
	}

	decayedVal, err := scoring.GeometricDecay(received, g.recordDecay, counter.lastUpdated)
	if err != nil {
		return counter, fmt.Errorf("could not recordDecay duplicate message counter gauge: %w", err)
	}
	counter.Gauge = decayedVal
	return counter, nil
}

// Get returns the current number of duplicate messages encountered from a peer. The counter is decayed before being
// returned.
// Args:
// - peerID: the peer ID of the peer in the GossipSub protocol.
//
// Returns:
// - float64: updated value for the duplicate message tracker.
// - bool: true if the counter was initialized, false otherwise.
// - error: if any error was encountered during record adjustments in cache.
// No errors are expected during normal operation.
func (g *GossipSubDuplicateMessageTrackerCache) Get(peerId peer.ID) (float64, bool, error) {
	if g.init(peerId) {
		return 0, true, nil
	}
	var err error
	entityID := flow.HashToID([]byte(peerId))
	adjustedEntity, adjusted := g.c.Adjust(entityID, func(entity flow.Entity) flow.Entity {
		// perform recordDecay on gauge value
		record := duplicateMessagesCounter(entity)
		entity, err = g.decayAdjustment(record)
		return entity
	})
	if err != nil {
		return 0, false, fmt.Errorf("unexpected error while applying recordDecay adjustment for peerID %s: %w", peerId, err)
	}
	if !adjusted {
		return 0, false, fmt.Errorf("unexpected error record not found for  peerID %s, even after an init attempt", peerId)
	}

	record := duplicateMessagesCounter(adjustedEntity)
	return record.Gauge, true, nil
}

// DuplicateMessagesCounter cache record that keeps track of the amount of duplicate messages received from a peer.
type DuplicateMessagesCounter struct {
	// ID the entity ID.
	Id flow.Identifier
	// Gauge the number of duplicate messages.
	Gauge       float64
	lastUpdated time.Time
}

func NewDuplicateMessagesCounter(id flow.Identifier) DuplicateMessagesCounter {
	return DuplicateMessagesCounter{
		Id:          id,
		Gauge:       0.0,
		lastUpdated: time.Now(),
	}
}

var _ flow.Entity = (*DuplicateMessagesCounter)(nil)

func (d DuplicateMessagesCounter) ID() flow.Identifier {
	return d.Id
}

func (d DuplicateMessagesCounter) Checksum() flow.Identifier {
	return d.Id
}

// duplicateMessagesCounter infers the DuplicateMessagesCounter type from the flow entity provided, panics if the wrong type is encountered.
func duplicateMessagesCounter(entity flow.Entity) DuplicateMessagesCounter {
	record, ok := entity.(DuplicateMessagesCounter)
	if !ok {
		// sanity check
		// This should never happen, because the cache only contains DuplicateMessagesCounter entities.
		panic(fmt.Sprintf("invalid entity type, expected DuplicateMessagesCounter type, got: %T", entity))
	}
	return record
}
