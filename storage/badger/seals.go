// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Seals struct {
	db       *badger.DB
	payloads *Payloads
	cache    *Cache
}

func NewSeals(db *badger.DB) *Seals {

	store := func(sealID flow.Identifier, seal interface{}) error {
		return db.Update(operation.InsertSeal(sealID, seal.(*flow.Seal)))
	}

	retrieve := func(sealID flow.Identifier) (interface{}, error) {
		var seal flow.Seal
		err := db.View(operation.RetrieveSeal(sealID, &seal))
		return &seal, err
	}

	s := &Seals{
		db:       db,
		payloads: NewPayloads(db),
		cache:    newCache(withLimit(1000), withStore(store), withRetrieve(retrieve)),
	}

	return s
}

func (s *Seals) Store(seal *flow.Seal) error {
	return s.cache.Put(seal.ID(), seal)
}

func (s *Seals) ByID(sealID flow.Identifier) (*flow.Seal, error) {
	seal, err := s.cache.Get(sealID)
	if err != nil {
		return nil, err
	}
	return seal.(*flow.Seal), nil
}

func (s *Seals) ByBlockID(blockID flow.Identifier) ([]*flow.Seal, error) {
	payload, err := s.payloads.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve block payload: %w", err)
	}
	return payload.Seals, nil
}

func (s *Seals) BySealedID(sealedID flow.Identifier) (*flow.Seal, error) {
	var sealID flow.Identifier
	err := s.db.View(operation.LookupBlockSeal(sealedID, &sealID))
	if err != nil {
		return nil, fmt.Errorf("could not look up seal for sealed: %w", err)
	}
	return s.ByID(sealID)
}
