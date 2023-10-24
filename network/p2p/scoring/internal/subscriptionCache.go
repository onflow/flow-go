package internal

import (
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

var ErrTopicRecordNotFound = fmt.Errorf("topic record not found")

type SubscriptionCache struct {
	c            *stdmap.Backend
	currentCycle atomic.Uint64
}

func (s *SubscriptionCache) GetSubscribedTopics(pid peer.ID) ([]string, bool) {
	e, ok := s.c.ByID(flow.MakeID(pid))
	if !ok {
		return nil, false
	}
	return e.(*SubscriptionRecordEntity).Topics, true
}

func (s *SubscriptionCache) MoveToNextUpdateCycle() {
	s.currentCycle.Inc()
}

func (s *SubscriptionCache) AddTopicForPeer(pid peer.ID, topic string) ([]string, error) {
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

func (s *SubscriptionCache) addTopicForPeer(entityId flow.Identifier, topic string) ([]string, error) {
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
