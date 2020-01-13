// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Seals implements the block seals memory pool of the consensus nodes,
// used to store block seals.
type Seals struct {
	*backend
}

// NewSeals creates a new memory pool for block seals.
func NewSeals() (*Seals, error) {
	s := &Seals{
		backend: newBackend(),
	}

	return s, nil
}

// Add adds an block seal to the mempool.
func (s *Seals) Add(seal *flow.Seal) error {
	return s.backend.Add(seal)
}

// Get returns the block seal with the given ID from the mempool.
func (s *Seals) Get(sealID flow.Identifier) (*flow.Seal, error) {
	entity, err := s.backend.Get(sealID)
	if err != nil {
		return nil, err
	}
	seal, ok := entity.(*flow.Seal)
	if !ok {
		panic(fmt.Sprintf("invalid entity in seal pool (%T)", entity))
	}
	return seal, nil
}

// All returns all block seals from the pool.
func (s *Seals) All() []*flow.Seal {
	entities := s.backend.All()
	seals := make([]*flow.Seal, 0, len(entities))
	for _, entity := range entities {
		seal, ok := entity.(*flow.Seal)
		if !ok {
			panic(fmt.Sprintf("invalid entity in seal pool (%T)", entity))
		}
		seals = append(seals, seal)
	}
	return seals
}
