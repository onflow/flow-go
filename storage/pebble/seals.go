package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

type Seals struct {
	db    *pebble.DB
	cache *Cache[flow.Identifier, *flow.Seal]
}

func NewSeals(collector module.CacheMetrics, db *pebble.DB) *Seals {

	store := func(sealID flow.Identifier, seal *flow.Seal) func(rw storage.PebbleReaderBatchWriter) error {
		return storage.OnlyWriter(operation.InsertSeal(sealID, seal))
	}

	retrieve := func(sealID flow.Identifier) func(pebble.Reader) (*flow.Seal, error) {
		return func(tx pebble.Reader) (*flow.Seal, error) {
			var seal flow.Seal
			err := operation.RetrieveSeal(sealID, &seal)(tx)
			return &seal, err
		}
	}

	s := &Seals{
		db: db,
		cache: newCache(collector, metrics.ResourceSeal,
			withLimit[flow.Identifier, *flow.Seal](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return s
}

func (s *Seals) storeTx(seal *flow.Seal) func(storage.PebbleReaderBatchWriter) error {
	return s.cache.PutPebble(seal.ID(), seal)
}

func (s *Seals) retrieveTx(sealID flow.Identifier) func(pebble.Reader) (*flow.Seal, error) {
	return func(tx pebble.Reader) (*flow.Seal, error) {
		val, err := s.cache.Get(sealID)(tx)
		if err != nil {
			return nil, err
		}
		return val, err
	}
}

func (s *Seals) Store(seal *flow.Seal) error {
	return operation.WithReaderBatchWriter(s.db, s.storeTx(seal))
}

func (s *Seals) ByID(sealID flow.Identifier) (*flow.Seal, error) {
	return s.retrieveTx(sealID)(s.db)
}

// HighestInFork retrieves the highest seal that was included in the
// fork up to (and including) blockID. This method should return a seal
// for any block known to the node. Returns storage.ErrNotFound if
// blockID is unknown.
func (s *Seals) HighestInFork(blockID flow.Identifier) (*flow.Seal, error) {
	var sealID flow.Identifier
	err := operation.LookupLatestSealAtBlock(blockID, &sealID)(s.db)
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
	err := operation.LookupBySealedBlockID(blockID, &sealID)(s.db)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve seal for block %x: %w", blockID, err)
	}
	return s.ByID(sealID)
}
