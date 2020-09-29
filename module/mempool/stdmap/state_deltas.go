// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

// Deltas implements the block deltas memory pool of the consensus nodes,
// used to store block deltas.
type Deltas struct {
	*Backend
}

// NewDeltas creates a new memory pool for block deltas.
func NewDeltas(limit uint, opts ...OptionFunc) (*Deltas, error) {
	s := &Deltas{
		Backend: NewBackend(append(opts, WithLimit(limit))...),
	}

	return s, nil
}

// Add adds an block delta to the mempool.
func (s *Deltas) Add(delta *messages.ExecutionStateDelta) bool {
	added := s.Backend.Add(delta)
	return added
}

// Rem will remove a delta by ID.
func (s *Deltas) Rem(deltaID flow.Identifier) bool {
	removed := s.Backend.Rem(deltaID)
	return removed
}

// ByID returns the block delta with the given ID from the mempool.
func (s *Deltas) ByID(deltaID flow.Identifier) (*messages.ExecutionStateDelta, bool) {
	entity, exists := s.Backend.ByID(deltaID)
	if !exists {
		return nil, false
	}
	delta := entity.(*messages.ExecutionStateDelta)
	return delta, true
}

// All returns all block deltas from the pool.
func (s *Deltas) All() []*messages.ExecutionStateDelta {
	entities := s.Backend.All()
	deltas := make([]*messages.ExecutionStateDelta, 0, len(entities))
	for _, entity := range entities {
		deltas = append(deltas, entity.(*messages.ExecutionStateDelta))
	}
	return deltas
}
