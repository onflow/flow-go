// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type Seals struct {
	db    *badger.DB
	cache *Cache
}

func NewSeals(collector module.CacheMetrics, db *badger.DB) *Seals {

	store := func(key interface{}, val interface{}) func(*transaction.Tx) error {
		sealID := key.(flow.Identifier)
		seal := val.(*flow.Seal)
		return transaction.WithTx(operation.SkipDuplicates(operation.InsertSeal(sealID, seal)))
	}

	retrieve := func(key interface{}) func(*badger.Txn) (interface{}, error) {
		sealID := key.(flow.Identifier)
		var seal flow.Seal
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveSeal(sealID, &seal)(tx)
			return &seal, err
		}
	}

	s := &Seals{
		db: db,
		cache: newCache(collector, metrics.ResourceSeal,
			withLimit(flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return s
}

func (s *Seals) storeTx(seal *flow.Seal) func(*transaction.Tx) error {
	return s.cache.PutTx(seal.ID(), seal)
}

func (s *Seals) retrieveTx(sealID flow.Identifier) func(*badger.Txn) (*flow.Seal, error) {
	return func(tx *badger.Txn) (*flow.Seal, error) {
		val, err := s.cache.Get(sealID)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*flow.Seal), err
	}
}

func (s *Seals) Store(seal *flow.Seal) error {
	return operation.RetryOnConflictTx(s.db, transaction.Update, s.storeTx(seal))
}

func (s *Seals) ByID(sealID flow.Identifier) (*flow.Seal, error) {
	tx := s.db.NewTransaction(false)
	defer tx.Discard()
	return s.retrieveTx(sealID)(tx)
}

// HighestInFork retrieves the highest seal that was included in the
// fork up to (and including) blockID. This method should return a seal
// for any block known to the node. Returns storage.ErrNotFound if
// blockID is unknown.
func (s *Seals) HighestInFork(blockID flow.Identifier) (*flow.Seal, error) {
	var sealID flow.Identifier
	err := s.db.View(operation.LookupLatestSealAtBlock(blockID, &sealID))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve seal for fork with head %x: %w", blockID, err)
	}
	return s.ByID(sealID)
}

// FinalizedSealForBlock returns the seal for the given block, only if that seal
// has been included in a finalized block.
// Returns storage.ErrNotFound if the block is unknown or unsealed.
func (s *Seals) FinalizedSealForBlock(blockID flow.Identifier) (*flow.Seal, error) {
	var sealID flow.Identifier
	err := s.db.View(operation.LookupBySealedBlockID(blockID, &sealID))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve seal for block %x: %w", blockID, err)
	}
	return s.ByID(sealID)
}
