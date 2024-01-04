package internal

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

// SubscriptionRecordCache manages the subscription records of peers in a network.
// It uses a currentCycle counter to track the update cycles of the cache, ensuring the relevance of subscription data.
type SubscriptionRecordCache struct {
	c *stdmap.Backend

	// currentCycle is an atomic counter used to track the update cycles of the subscription cache.
	// It plays a critical role in maintaining the cache's data relevance and coherence.
	// Each increment of currentCycle represents a new update cycle, signifying the cache's transition to a new state
	// where only the most recent and relevant subscriptions are maintained. This design choice ensures that the cache
	// does not retain stale or outdated subscription information, thereby reflecting the dynamic nature of peer
	// subscriptions in the network. It is incremented every time the subscription cache is updated, either with new
	// topic subscriptions or other update operations.
	// The currentCycle is incremented atomically and externally by calling the MoveToNextUpdateCycle() function.
	// This is called by the module that uses the subscription provider cache signaling that whatever updates it has
	// made to the cache so far can be considered out-of-date, and the new updates to the cache records should
	// overwrite the old ones.
	currentCycle atomic.Uint64
}

// NewSubscriptionRecordCache creates a new subscription cache with the given size limit.
// Args:
// - sizeLimit: the size limit of the cache.
// - logger: the logger to use for logging.
// - collector: the metrics collector to use for collecting metrics.
func NewSubscriptionRecordCache(sizeLimit uint32,
	logger zerolog.Logger,
	collector module.HeroCacheMetrics) *SubscriptionRecordCache {
	backData := herocache.NewCache(sizeLimit,
		herocache.DefaultOversizeFactor,
		heropool.LRUEjection,
		logger.With().Str("mempool", "subscription-records").Logger(),
		collector)

	return &SubscriptionRecordCache{
		c:            stdmap.NewBackend(stdmap.WithBackData(backData)),
		currentCycle: *atomic.NewUint64(0),
	}
}

// GetSubscribedTopics returns the list of topics a peer is subscribed to.
// Returns:
// - []string: the list of topics the peer is subscribed to.
// - bool: true if there is a record for the peer, false otherwise.
func (s *SubscriptionRecordCache) GetSubscribedTopics(pid peer.ID) ([]string, bool) {
	e, ok := s.c.ByID(flow.MakeID(pid))
	if !ok {
		return nil, false
	}
	return e.(SubscriptionRecordEntity).Topics, true
}

// MoveToNextUpdateCycle moves the subscription cache to the next update cycle.
// A new update cycle is started when the subscription cache is first created, and then every time the subscription cache
// is updated. The update cycle is used to keep track of the last time the subscription cache was updated. It is used to
// implement a notion of time in the subscription cache.
// When the update cycle is moved forward, it means that all the updates made to the subscription cache so far are
// considered out-of-date, and the new updates to the cache records should overwrite the old ones.
// The expected behavior is that the update cycle is moved forward by the module that uses the subscription provider once
// per each update on the "entire" cache (and not per each update on a single record).
// In other words, assume a cache with 3 records: A, B, and C. If the module updates record A, then record B, and then
// record C, the module should move the update cycle forward only once after updating record C, and then update record A
// B, and C again. If the module moves the update cycle forward after updating record A, then again after updating
// record B, and then again after updating record C, the cache will be in an inconsistent state.
// Returns:
// - uint64: the current update cycle.
func (s *SubscriptionRecordCache) MoveToNextUpdateCycle() uint64 {
	s.currentCycle.Inc()
	return s.currentCycle.Load()
}

// AddWithInitTopicForPeer appends a topic to the list of topics a peer is subscribed to. If the peer is not subscribed to any
// topics yet, a new record is created.
// If the last update cycle is older than the current cycle, the list of topics for the peer is first cleared, and then
// the topic is added to the list. This is to ensure that the list of topics for a peer is always up to date.
// Args:
// - pid: the peer id of the peer.
// - topic: the topic to add.
// Returns:
// - []string: the list of topics the peer is subscribed to after the update.
// - error: an error if the update failed; any returned error is an irrecoverable error and indicates a bug or misconfiguration.
// Implementation must be thread-safe.
func (s *SubscriptionRecordCache) AddWithInitTopicForPeer(pid peer.ID, topic string) ([]string, error) {
	// ensuring atomic init-and-adjust operation.

	// first, we try to optimistically adjust the record assuming that the record already exists.
	entityId := flow.MakeID(pid)
	initLogic := func() flow.Entity {
		return SubscriptionRecordEntity{
			entityId:         entityId,
			PeerID:           pid,
			Topics:           make([]string, 0),
			LastUpdatedCycle: s.currentCycle.Load(),
		}
	}
	var rErr error
	adjustLogic := func(entity flow.Entity) flow.Entity {
		record, ok := entity.(SubscriptionRecordEntity)
		if !ok {
			// sanity check
			// This should never happen, because the cache only contains SubscriptionRecordEntity entities.
			panic(fmt.Sprintf("invalid entity type, expected SubscriptionRecordEntity type, got: %T", entity))
		}

		currentCycle := s.currentCycle.Load()
		if record.LastUpdatedCycle > currentCycle {
			// sanity check
			// This should never happen, because the update cycle must be moved forward before adding a topic.
			panic(fmt.Sprintf("invalid last updated cycle, expected <= %d, got: %d", currentCycle, record.LastUpdatedCycle))
		}
		if record.LastUpdatedCycle < currentCycle {
			// This record was not updated in the current cycle, so we can wipe its topics list (topic list is only
			// valid for the current cycle).
			record.Topics = make([]string, 0)
		}
		// check if the topic already exists; if it does, we do not need to update the record.
		for _, t := range record.Topics {
			if t == topic {
				// topic already exists
				return record
			}
		}
		record.LastUpdatedCycle = currentCycle
		record.Topics = append(record.Topics, topic)

		// Return the adjusted record.
		return record
	}
	adjustedEntity, adjusted := s.c.AdjustWithInit(entityId, adjustLogic, initLogic)
	if rErr != nil {
		return nil, fmt.Errorf("failed to adjust record with error: %w", rErr)
	}
	if !adjusted {
		return nil, fmt.Errorf("failed to adjust record, entity not found")
	}

	return adjustedEntity.(SubscriptionRecordEntity).Topics, nil
}
