// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// Seals implements the block seals memory pool of the consensus nodes,
// used to store block seals.
type Seals struct {
	*Backend
}

// NewSeals creates a new memory pool for block seals.
func NewSeals(limit uint, opts ...OptionFunc) (*Seals, error) {
	s := &Seals{
		Backend: NewBackend(append(opts, WithLimit(limit))...),
	}

	return s, nil
}

// Add adds an block seal to the mempool.
func (s *Seals) Add(seal *flow.SealContainer) bool {
	added := s.Backend.Add(seal)
	return added
}

// Rem will remove a seal by ID.
func (s *Seals) Rem(sealID flow.Identifier) bool {
	removed := s.Backend.Rem(sealID)
	return removed
}

// ByID returns the block seal with the given ID from the mempool.
func (s *Seals) ByID(sealID flow.Identifier) (*flow.SealContainer, bool) {
	entity, exists := s.Backend.ByID(sealID)
	if !exists {
		return nil, false
	}
	seal := entity.(*flow.SealContainer)
	return seal, true
}

// All returns all block seals from the pool.
func (s *Seals) All() []*flow.SealContainer {
	entities := s.Backend.All()
	seals := make([]*flow.SealContainer, 0, len(entities))
	for _, entity := range entities {
		seals = append(seals, entity.(*flow.SealContainer))
	}
	return seals
}
