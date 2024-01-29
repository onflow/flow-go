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
	"github.com/onflow/flow-go/network/p2p/scoring"
)

// DuplicateMessageTrackerCache is a cache used to store the current count of duplicate messages detected
// from a peer. This count is utilized to calculate a penalty for duplicate messages, which is then applied
// to the peer's application-specific score. The duplicate message tracker decays over time to prevent perpetual
// penalization of a peer.
type DuplicateMessageTrackerCache struct {
	// the in-memory and thread-safe cache for storing the spam records of peers.
	c     *stdmap.Backend
	decay float64
	// skipDecayThreshold The threshold for which when the counter is below this value, the decay function will not be called
	skipDecayThreshold float64
}

// NewDuplicateMessageTrackerCache returns a new HeroCache-based duplicate message counter cache.
// Args:
//
//	sizeLimit: the maximum number of entries that can be stored in the cache.
//	decay: the record decay.
//	logger: the logger to be used by the cache.
//	collector: the metrics collector to be used by the cache.
//
// Returns:
//   - *DuplicateMessageTrackerCache: the newly created cache with a HeroCache-based backend.
func NewDuplicateMessageTrackerCache(sizeLimit uint32, decay, skipDecayThreshold float64, logger zerolog.Logger, collector module.HeroCacheMetrics) *DuplicateMessageTrackerCache {
	backData := herocache.NewCache(sizeLimit,
		herocache.DefaultOversizeFactor,
		// we should not evict any record from the cache,
		// eviction will open the node to spam attacks by malicious peers to erase their duplicate message counters reducing the overall penalty applied application specific score.
		heropool.LRUEjection,
		logger.With().Str("mempool", "gossipsub=duplicate-message-counter-cache").Logger(),
		collector)
	return &DuplicateMessageTrackerCache{
		decay:              decay,
		skipDecayThreshold: skipDecayThreshold,
		c:                  stdmap.NewBackend(stdmap.WithBackData(backData)),
	}
}

// DuplicateMessageReceived applies an adjustment that increments the number of duplicate messages received by a peer.
// Returns number of duplicate messages received after the adjustment. The record is initialized before
// the adjustment func is applied that will increment the counter value.
//   - exception only in cases of internal data inconsistency or bugs. No errors are expected.
func (d *DuplicateMessageTrackerCache) DuplicateMessageReceived(peerID peer.ID) (float64, error) {
	var err error
	adjustFunc := func(entity flow.Entity) flow.Entity {
		entity, err = d.decayAdjustment(entity) // first decay the record
		if err != nil {
			return entity
		}
		return d.incrementAdjustment(entity) // then increment the record
	}

	entityId := makeId(peerID)
	adjustedEntity, adjusted := d.c.AdjustWithInit(entityId, adjustFunc, func() flow.Entity {
		return NewDuplicateMessagesCounter(entityId)
	})

	if err != nil {
		return 0, fmt.Errorf("unexpected error while applying decay and increment adjustments for peer %s: %w", peerID, err)
	}

	if !adjusted {
		return 0, fmt.Errorf("adjustment failed for peer %s", peerID)
	}

	record := mustBeDuplicateMessagesCounterEntity(adjustedEntity)

	return record.Value, nil
}

// GetWithInit returns the current number of duplicate messages received from a peer.
// The record is initialized before the count is returned.
// Before the counter value is returned it is decayed using the configured decay function.
// Returns the record and true if the record exists, nil and false otherwise.
// Args:
// - peerID: peerID of the remote peer.
// Returns:
// - The duplicate messages counter value after the decay and true if the record exists, 0 and false otherwise.
// No errors are expected during normal operation.
func (d *DuplicateMessageTrackerCache) GetWithInit(peerID peer.ID) (float64, bool, error) {
	var err error
	adjustLogic := func(entity flow.Entity) flow.Entity {
		// perform decay on gauge value
		entity, err = d.decayAdjustment(entity)
		return entity
	}

	entityId := makeId(peerID)
	adjustedEntity, adjusted := d.c.AdjustWithInit(entityId, adjustLogic, func() flow.Entity {
		return newDuplicateMessagesCounter(entityId)
	})
	if err != nil {
		return 0, false, fmt.Errorf("unexpected error while applying decay adjustment for peer %s: %w", peerID, err)
	}
	if !adjusted {
		return 0, false, fmt.Errorf("decay adjustment failed for peer %s", peerID)
	}

	counter := mustBeDuplicateMessagesCounterEntity(adjustedEntity)

	return counter.Value, true, nil
}

// incrementAdjustment performs a cache adjustment that increments the guage for the duplicateMessagesCounterEntity
func (d *DuplicateMessageTrackerCache) incrementAdjustment(entity flow.Entity) flow.Entity {
	counter := mustBeDuplicateMessagesCounterEntity(entity)
	counter.Value++
	counter.lastUpdated = time.Now()
	// Return the adjusted counter.
	return counter
}

// decayAdjustment performs geometric recordDecay on the duplicate message counter gauge of a peer. This ensures a peer is not penalized forever.
// All errors returned from this function are unexpected and irrecoverable.
func (d *DuplicateMessageTrackerCache) decayAdjustment(entity flow.Entity) (flow.Entity, error) {
	counter := mustBeDuplicateMessagesCounterEntity(entity)
	duplicateMessages := counter.Value
	if duplicateMessages == 0 {
		return counter, nil
	}

	if duplicateMessages < d.skipDecayThreshold {
		counter.Value = 0
		return counter, nil
	}

	decayedVal, err := scoring.GeometricDecay(duplicateMessages, d.decay, counter.lastUpdated)
	if err != nil {
		return counter, fmt.Errorf("could not decay duplicate message counter: %w", err)
	}

	if decayedVal > duplicateMessages {
		return counter, fmt.Errorf("unexpected recordDecay value %f for duplicate message counter gauge %f", decayedVal, duplicateMessages)
	}

	counter.Value = decayedVal
	counter.lastUpdated = time.Now()
	// Return the adjusted counter.
	return counter, nil
}
