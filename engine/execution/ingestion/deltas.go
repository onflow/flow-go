package ingestion

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

type Deltas struct {
	*stdmap.Backend
}

// NewDeltas creates a new memory pool for state deltas
func NewDeltas(limit uint, opts ...stdmap.OptionFunc) (*Deltas, error) {
	s := &Deltas{
		Backend: stdmap.NewBackend(append(opts, stdmap.WithLimit(limit))...),
	}

	return s, nil
}

// Add adds an state deltas to the mempool.
func (s *Deltas) Add(delta *messages.ExecutionStateDelta) bool {
	return s.Backend.Add(delta)
}

// Rem will remove a deltas by ID.
func (s *Deltas) Rem(deltaID flow.Identifier) bool {
	removed := s.Backend.Rem(deltaID)
	return removed
}

// ByID returns the state deltas with the given ID from the mempool.
// delta's ID is also blockID
func (s *Deltas) ByID(deltaID flow.Identifier) (*messages.ExecutionStateDelta, bool) {
	entity, exists := s.Backend.ByID(deltaID)
	if !exists {
		return nil, false
	}
	delta := entity.(*messages.ExecutionStateDelta)
	return delta, true
}

// All returns all block Deltass from the pool.
func (s *Deltas) All() []*messages.ExecutionStateDelta {
	entities := s.Backend.All()
	deltas := make([]*messages.ExecutionStateDelta, 0, len(entities))
	for _, entity := range entities {
		deltas = append(deltas, entity.(*messages.ExecutionStateDelta))
	}
	return deltas
}
