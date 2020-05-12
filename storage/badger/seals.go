// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Seals struct {
	db    *badger.DB
	cache *Cache
}

func NewSeals(collector module.CacheMetrics, db *badger.DB) *Seals {

	store := func(sealID flow.Identifier, seal interface{}) error {
		return operation.RetryOnConflict(db.Update, operation.SkipDuplicates(operation.InsertSeal(sealID, seal.(*flow.Seal))))
	}

	retrieve := func(sealID flow.Identifier) (interface{}, error) {
		var seal flow.Seal
		err := db.View(operation.RetrieveSeal(sealID, &seal))
		return &seal, err
	}

	s := &Seals{
		db: db,
		cache: newCache(collector,
			withLimit(flow.DefaultTransactionExpiry),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceSeal),
		),
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

func (s *Seals) ByBlockID(blockID flow.Identifier) (*flow.Seal, error) {
	var sealID flow.Identifier
	err := s.db.View(operation.LookupBlockSeal(blockID, &sealID))
	if err != nil {
		return nil, fmt.Errorf("could not look up seal for sealed: %w", err)
	}
	return s.ByID(sealID)
}
