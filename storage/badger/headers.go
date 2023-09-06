// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// Headers implements a simple read-only header storage around a badger DB.
type Headers struct {
	db          *badger.DB
	cache       *Cache[flow.Identifier, *flow.Header]
	heightCache *Cache[uint64, flow.Identifier]
}

func NewHeaders(collector module.CacheMetrics, db *badger.DB) *Headers {

	store := func(blockID flow.Identifier, header *flow.Header) func(*transaction.Tx) error {
		return transaction.WithTx(operation.InsertHeader(blockID, header))
	}

	// CAUTION: should only be used to index FINALIZED blocks by their
	// respective height
	storeHeight := func(height uint64, id flow.Identifier) func(*transaction.Tx) error {
		return transaction.WithTx(operation.IndexBlockHeight(height, id))
	}

	retrieve := func(blockID flow.Identifier) func(tx *badger.Txn) (*flow.Header, error) {
		var header flow.Header
		return func(tx *badger.Txn) (*flow.Header, error) {
			err := operation.RetrieveHeader(blockID, &header)(tx)
			return &header, err
		}
	}

	retrieveHeight := func(height uint64) func(tx *badger.Txn) (flow.Identifier, error) {
		return func(tx *badger.Txn) (flow.Identifier, error) {
			var id flow.Identifier
			err := operation.LookupBlockHeight(height, &id)(tx)
			return id, err
		}
	}

	h := &Headers{
		db: db,
		cache: newCache[flow.Identifier, *flow.Header](collector, metrics.ResourceHeader,
			withLimit[flow.Identifier, *flow.Header](4*flow.DefaultTransactionExpiry),
			withStore(store),
			withRetrieve(retrieve)),

		heightCache: newCache[uint64, flow.Identifier](collector, metrics.ResourceFinalizedHeight,
			withLimit[uint64, flow.Identifier](4*flow.DefaultTransactionExpiry),
			withStore(storeHeight),
			withRetrieve(retrieveHeight)),
	}

	return h
}

func (h *Headers) storeTx(header *flow.Header) func(*transaction.Tx) error {
	return h.cache.PutTx(header.ID(), header)
}

func (h *Headers) retrieveTx(blockID flow.Identifier) func(*badger.Txn) (*flow.Header, error) {
	return func(tx *badger.Txn) (*flow.Header, error) {
		val, err := h.cache.Get(blockID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

// results in `storage.ErrNotFound` for unknown height
func (h *Headers) retrieveIdByHeightTx(height uint64) func(*badger.Txn) (flow.Identifier, error) {
	return func(tx *badger.Txn) (flow.Identifier, error) {
		blockID, err := h.heightCache.Get(height)(tx)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("failed to retrieve block ID for height %d: %w", height, err)
		}
		return blockID, nil
	}
}

func (h *Headers) Store(header *flow.Header) error {
	return operation.RetryOnConflictTx(h.db, transaction.Update, h.storeTx(header))
}

func (h *Headers) ByBlockID(blockID flow.Identifier) (*flow.Header, error) {
	tx := h.db.NewTransaction(false)
	defer tx.Discard()
	return h.retrieveTx(blockID)(tx)
}

func (h *Headers) ByHeight(height uint64) (*flow.Header, error) {
	tx := h.db.NewTransaction(false)
	defer tx.Discard()

	blockID, err := h.retrieveIdByHeightTx(height)(tx)
	if err != nil {
		return nil, err
	}
	return h.retrieveTx(blockID)(tx)
}

// Exists returns true if a header with the given ID has been stored.
// No errors are expected during normal operation.
func (h *Headers) Exists(blockID flow.Identifier) (bool, error) {
	// if the block is in the cache, return true
	if ok := h.cache.IsCached(blockID); ok {
		return ok, nil
	}
	// otherwise, check badger store
	var exists bool
	err := h.db.View(operation.BlockExists(blockID, &exists))
	if err != nil {
		return false, fmt.Errorf("could not check existence: %w", err)
	}
	return exists, nil
}

// BlockIDByHeight returns the block ID that is finalized at the given height. It is an optimized
// version of `ByHeight` that skips retrieving the block. Expected errors during normal operations:
//   - `storage.ErrNotFound` if no finalized block is known at given height.
func (h *Headers) BlockIDByHeight(height uint64) (flow.Identifier, error) {
	tx := h.db.NewTransaction(false)
	defer tx.Discard()

	blockID, err := h.retrieveIdByHeightTx(height)(tx)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not lookup block id by height %d: %w", height, err)
	}
	return blockID, nil
}

func (h *Headers) ByParentID(parentID flow.Identifier) ([]*flow.Header, error) {
	var blockIDs flow.IdentifierList
	err := h.db.View(procedure.LookupBlockChildren(parentID, &blockIDs))
	if err != nil {
		return nil, fmt.Errorf("could not look up children: %w", err)
	}
	headers := make([]*flow.Header, 0, len(blockIDs))
	for _, blockID := range blockIDs {
		header, err := h.ByBlockID(blockID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve child (%x): %w", blockID, err)
		}
		headers = append(headers, header)
	}
	return headers, nil
}

func (h *Headers) FindHeaders(filter func(header *flow.Header) bool) ([]flow.Header, error) {
	blocks := make([]flow.Header, 0, 1)
	err := h.db.View(operation.FindHeaders(filter, &blocks))
	return blocks, err
}

// RollbackExecutedBlock update the executed block header to the given header.
// only useful for execution node to roll back executed block height
func (h *Headers) RollbackExecutedBlock(header *flow.Header) error {
	return operation.RetryOnConflict(h.db.Update, func(txn *badger.Txn) error {
		var blockID flow.Identifier
		err := operation.RetrieveExecutedBlock(&blockID)(txn)
		if err != nil {
			return fmt.Errorf("cannot lookup executed block: %w", err)
		}

		var highest flow.Header
		err = operation.RetrieveHeader(blockID, &highest)(txn)
		if err != nil {
			return fmt.Errorf("cannot retrieve executed header: %w", err)
		}

		// only rollback if the given height is below the current executed height
		if header.Height >= highest.Height {
			return fmt.Errorf("cannot roolback. expect the target height %v to be lower than highest executed height %v, but actually is not",
				header.Height, highest.Height,
			)
		}

		err = operation.UpdateExecutedBlock(header.ID())(txn)
		if err != nil {
			return fmt.Errorf("cannot update highest executed block: %w", err)
		}

		return nil
	})
}
