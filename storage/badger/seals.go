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

	store := func(sealID flow.Identifier, v interface{}) func(*badger.Txn) error {
		seal := v.(*flow.Seal)
		return operation.SkipDuplicates(operation.InsertSeal(sealID, seal))
	}

	retrieve := func(sealID flow.Identifier) func(*badger.Txn) (interface{}, error) {
		var seal flow.Seal
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveSeal(sealID, &seal)(tx)
			return &seal, err
		}
	}

	s := &Seals{
		db: db,
		cache: newCache(collector,
			withLimit(flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceSeal)),
	}

	return s
}

func (s *Seals) storeTx(seal *flow.Seal) func(*badger.Txn) error {
	return s.cache.Put(seal.ID(), seal)
}

func (s *Seals) retrieveTx(sealID flow.Identifier) func(*badger.Txn) (*flow.Seal, error) {
	return func(tx *badger.Txn) (*flow.Seal, error) {
		v, err := s.cache.Get(sealID)(tx)
		if err != nil {
			return nil, err
		}
		return v.(*flow.Seal), err
	}
}

func (s *Seals) Store(seal *flow.Seal) error {
	return operation.RetryOnConflict(s.db.Update, s.storeTx(seal))
}

func (s *Seals) ByID(sealID flow.Identifier) (*flow.Seal, error) {
	return s.retrieveTx(sealID)(s.db.NewTransaction(false))
}

func (s *Seals) ByBlockID(blockID flow.Identifier) (*flow.Seal, error) {
	var sealID flow.Identifier
	err := s.db.View(operation.LookupBlockSeal(blockID, &sealID))
	if err != nil {
		return nil, fmt.Errorf("could not look up seal for sealed: %w", err)
	}
	return s.ByID(sealID)
}
