// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
)

// Seals implements the block seals memory pool of the consensus nodes,
// used to store block seals.
type Seals struct {
	*Backend
	byPrevious map[string]flow.Identifier
}

// NewSeals creates a new memory pool for block seals.
func NewSeals(limit uint) (*Seals, error) {
	s := &Seals{
		Backend:    NewBackend(WithLimit(limit)),
		byPrevious: make(map[string]flow.Identifier),
	}

	return s, nil
}

// Add adds an block seal to the mempool.
func (s *Seals) Add(seal *flow.Seal) error {
	return s.Backend.Run(func(backend *Backdata) error {
		err := backend.Add(seal)
		if err != nil {
			return err
		}
		s.byPrevious[string(seal.PreviousState)] = seal.ID()
		return nil
	})
}

// ByID returns the block seal with the given ID from the mempool.
func (s *Seals) ByID(sealID flow.Identifier) (*flow.Seal, error) {
	entity, err := s.Backend.ByID(sealID)
	if err != nil {
		return nil, err
	}
	seal, ok := entity.(*flow.Seal)
	if !ok {
		panic(fmt.Sprintf("invalid entity in seal pool (%T)", entity))
	}
	return seal, nil
}

// ByPreviousState returns the block seal associated with the given parent state
// commitment.
func (s *Seals) ByPreviousState(commit flow.StateCommitment) (*flow.Seal, error) {

	var seal *flow.Seal
	err := s.Backend.Run(func(backend *Backdata) error {
		sealID, ok := s.byPrevious[string(commit)]
		if !ok {
			return mempool.ErrEntityNotFound
		}
		byID, err := s.ByID(sealID)
		if err != nil {
			return err
		}

		seal = byID
		return nil
	})

	return seal, err
}

// All returns all block seals from the pool.
func (s *Seals) All() []*flow.Seal {
	entities := s.Backend.All()
	seals := make([]*flow.Seal, 0, len(entities))
	for _, entity := range entities {
		seals = append(seals, entity.(*flow.Seal))
	}
	return seals
}
