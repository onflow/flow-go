package store

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

type Seals struct {
	db    storage.DB
	cache *Cache[flow.Identifier, *flow.Seal]
}

func NewSeals(collector module.CacheMetrics, db storage.DB) *Seals {

	store := func(rw storage.ReaderBatchWriter, sealID flow.Identifier, seal *flow.Seal) error {
		return operation.InsertSeal(rw.Writer(), sealID, seal)
	}

	retrieve := func(r storage.Reader, sealID flow.Identifier) (*flow.Seal, error) {
		var seal flow.Seal
		err := operation.RetrieveSeal(r, sealID, &seal)
		return &seal, err
	}

	s := &Seals{
		db: db,
		cache: newCache[flow.Identifier, *flow.Seal](collector, metrics.ResourceSeal,
			withLimit[flow.Identifier, *flow.Seal](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return s
}

func (s *Seals) storeTx(rw storage.ReaderBatchWriter, seal *flow.Seal) error {
	return s.cache.PutTx(rw, seal.ID(), seal)
}

func (s *Seals) retrieveTx(sealID flow.Identifier) (*flow.Seal, error) {
	val, err := s.cache.Get(s.db.Reader(), sealID)
	if err != nil {
		return nil, err
	}
	return val, err
}

func (s *Seals) Store(seal *flow.Seal) error {
	return s.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return s.storeTx(rw, seal)
	})
}

func (s *Seals) ByID(sealID flow.Identifier) (*flow.Seal, error) {
	return s.retrieveTx(sealID)
}

// HighestInFork retrieves the highest seal that was included in the
// fork up to (and including) blockID. This method should return a seal
// for any block known to the node. Returns storage.ErrNotFound if
// blockID is unknown.
func (s *Seals) HighestInFork(blockID flow.Identifier) (*flow.Seal, error) {
	var sealID flow.Identifier
	err := operation.LookupLatestSealAtBlock(s.db.Reader(), blockID, &sealID)
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
	err := operation.LookupBySealedBlockID(s.db.Reader(), blockID, &sealID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve seal for block %x: %w", blockID, err)
	}
	return s.ByID(sealID)
}
