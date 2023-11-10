package internal

import (
	"errors"
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

var ErrTopicRecordNotFound = fmt.Errorf("topic record not found")

type SubscriptionRecordCache struct {
	c            *stdmap.Backend
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
// Returns:
// - uint64: the current update cycle.
func (s *SubscriptionRecordCache) MoveToNextUpdateCycle() uint64 {
	s.currentCycle.Inc()
	return s.currentCycle.Load()
}

// AddTopicForPeer appends a topic to the list of topics a peer is subscribed to. If the peer is not subscribed to any
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
func (s *SubscriptionRecordCache) AddTopicForPeer(pid peer.ID, topic string) ([]string, error) {
	// first, we try to optimistically adjust the record assuming that the record already exists.
	entityId := flow.MakeID(pid)
	topics, err := s.addTopicForPeer(entityId, topic)

	switch {
	case errors.Is(err, ErrTopicRecordNotFound):
		// if the record does not exist, we initialize the record and try to adjust it again.
		// Note: there is an edge case where the record is initialized by another goroutine between the two calls.
		// In this case, the init function is invoked twice, but it is not a problem because the underlying
		// cache is thread-safe. Hence, we do not need to synchronize the two calls. In such cases, one of the
		// two calls returns false, and the other call returns true. We do not care which call returns false, hence,
		// we ignore the return value of the init function.
		_ = s.c.Add(SubscriptionRecordEntity{
			entityId:         entityId,
			PeerID:           pid,
			Topics:           make([]string, 0),
			LastUpdatedCycle: s.currentCycle.Load(),
		})
		// as the record is initialized, the adjust attempt should not return an error, and any returned error
		// is an irrecoverable error and indicates a bug.
		return s.addTopicForPeer(entityId, topic)
	case err != nil:
		// if the adjust function returns an unexpected error on the first attempt, we return the error directly.
		return nil, err
	default:
		// if the adjust function returns no error, we return the updated list of topics.
		return topics, nil
	}
}

func (s *SubscriptionRecordCache) addTopicForPeer(entityId flow.Identifier, topic string) ([]string, error) {
	var rErr error
	updatedEntity, adjusted := s.c.Adjust(entityId, func(entity flow.Entity) flow.Entity {
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
	})

	if rErr != nil {
		return nil, fmt.Errorf("failed to adjust record: %w", rErr)
	}

	if !adjusted {
		return nil, ErrTopicRecordNotFound
	}

	return updatedEntity.(SubscriptionRecordEntity).Topics, nil
}
