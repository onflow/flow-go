package events

import (
	"context"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Store stores an indexed representation of events to support querying.
type Store interface {
	// Add adds one or events to the store.
	Add(ctx context.Context, blockNumber uint64, events ...flow.Event) error
	// Query searches for events in the store matching the given query.
	Query(ctx context.Context, query flow.EventQuery) ([]flow.Event, error)
}

// memStore implements an in-memory store for events. Events are indexed by
// block number and by ID
type memStore struct {
	byBlock map[uint64][]flow.Event
}

// NewMemStore returns a new in-memory Store implementation.
func NewMemStore() Store {
	return &memStore{
		byBlock: make(map[uint64][]flow.Event),
	}
}

func (s *memStore) Add(ctx context.Context, blockNumber uint64, events ...flow.Event) error {
	if s.byBlock[blockNumber] == nil {
		s.byBlock[blockNumber] = events
	} else {
		s.byBlock[blockNumber] = append(s.byBlock[blockNumber], events...)
	}

	return nil
}

func (s *memStore) Query(ctx context.Context, query flow.EventQuery) ([]flow.Event, error) {
	var events []flow.Event
	// Filter by block number first
	for i := query.StartBlock; i <= query.EndBlock; i++ {
		if s.byBlock[i] != nil {
			// Check for empty ID, which indicates no ID filtering
			if query.ID == "" {
				events = append(events, s.byBlock[i]...)
				continue
			}

			// Otherwise, only add events with matching ID
			for _, event := range s.byBlock[i] {
				if event.ID == query.ID {
					events = append(events, event)
				}
			}
		}
	}
	return events, nil
}
