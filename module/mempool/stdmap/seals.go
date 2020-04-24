// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
)

// Seals implements the block seals memory pool of the consensus nodes,
// used to store block seals.
type Seals struct {
	*Backend
	byBlock map[flow.Identifier]flow.Identifier
}

// NewSeals creates a new memory pool for block seals.
func NewSeals(limit uint) (*Seals, error) {
	s := &Seals{
		Backend: NewBackend(WithLimit(limit)),
		byBlock: make(map[flow.Identifier]flow.Identifier),
	}

	return s, nil
}

// Add adds an block seal to the mempool.
func (s *Seals) Add(seal *flow.Seal) error {
	err := s.Backend.Add(seal)
	if err != nil {
		return err
	}
	s.byBlock[seal.BlockID] = seal.ID()
	return nil
}

// Rem will remove a seal by ID.
func (s *Seals) Rem(sealID flow.Identifier) bool {
	entity, err := s.Backend.ByID(sealID)
	if err != nil {
		return false
	}
	_ = s.Backend.Rem(sealID)
	seal := entity.(*flow.Seal)
	delete(s.byBlock, seal.BlockID)
	return true
}

// ByID returns the block seal with the given ID from the mempool.
func (s *Seals) ByID(sealID flow.Identifier) (*flow.Seal, error) {
	entity, err := s.Backend.ByID(sealID)
	if err != nil {
		return nil, err
	}
	seal := entity.(*flow.Seal)
	return seal, nil
}

// ByBlocID returns the block seal associated with the given sealed block.
func (s *Seals) ByBlockID(blockID flow.Identifier) (*flow.Seal, error) {
	sealID, ok := s.byBlock[blockID]
	if !ok {
		return nil, mempool.ErrNotFound
	}
	return s.ByID(sealID)
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
