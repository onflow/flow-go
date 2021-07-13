// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// Headers implements a simple read-only header storage around a badger DB.
type Headers struct {
	db           *badger.DB
	cache        *Cache
	heightCache  *Cache
	chunkIDCache *Cache
}

func NewHeaders(collector module.CacheMetrics, db *badger.DB) *Headers {

	store := func(key interface{}, val interface{}) func(*transaction.Tx) error {
		blockID := key.(flow.Identifier)
		header := val.(*flow.Header)
		return transaction.WithTx(operation.InsertHeader(blockID, header))
	}

	// CAUTION: should only be used to index FINALIZED blocks by their
	// respective height
	storeHeight := func(key interface{}, val interface{}) func(*transaction.Tx) error {
		height := key.(uint64)
		id := val.(flow.Identifier)
		return transaction.WithTx(operation.IndexBlockHeight(height, id))
	}

	storeChunkID := func(key interface{}, val interface{}) func(*transaction.Tx) error {
		chunkID := key.(flow.Identifier)
		blockID := val.(flow.Identifier)
		return transaction.WithTx(operation.IndexBlockIDByChunkID(chunkID, blockID))
	}

	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		blockID := key.(flow.Identifier)
		var header flow.Header
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveHeader(blockID, &header)(tx)
			return &header, err
		}
	}

	retrieveHeight := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		height := key.(uint64)
		var id flow.Identifier
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.LookupBlockHeight(height, &id)(tx)
			return id, err
		}
	}

	retrieveChunkID := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		chunkID := key.(flow.Identifier)
		var blockID flow.Identifier
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.LookupBlockIDByChunkID(chunkID, &blockID)(tx)
			return blockID, err
		}
	}

	h := &Headers{
		db: db,
		cache: newCache(collector, metrics.ResourceHeader,
			withLimit(4*flow.DefaultTransactionExpiry),
			withStore(store),
			withRetrieve(retrieve)),

		heightCache: newCache(collector, metrics.ResourceFinalizedHeight,
			withLimit(4*flow.DefaultTransactionExpiry),
			withStore(storeHeight),
			withRetrieve(retrieveHeight)),
		chunkIDCache: newCache(collector, metrics.ResourceFinalizedHeight,
			withLimit(4*flow.DefaultTransactionExpiry),
			withStore(storeChunkID),
			withRetrieve(retrieveChunkID)),
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
		return val.(*flow.Header), nil
	}
}

func (h *Headers) retrieveIdByHeightTx(height uint64) func(*badger.Txn) (flow.Identifier, error) {
	return func(tx *badger.Txn) (flow.Identifier, error) {
		blockID, err := h.heightCache.Get(height)(tx)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("failed to retrieve block ID for height %d: %w", height, err)
		}
		return blockID.(flow.Identifier), nil
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

func (h *Headers) ByParentID(parentID flow.Identifier) ([]*flow.Header, error) {
	var blockIDs []flow.Identifier
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

func (h *Headers) IDByChunkID(chunkID flow.Identifier) (flow.Identifier, error) {
	tx := h.db.NewTransaction(false)
	defer tx.Discard()

	bID, err := h.chunkIDCache.Get(chunkID)(tx)
	if err != nil {
		return flow.Identifier{}, fmt.Errorf("could not look up by chunk id: %w", err)
	}
	return bID.(flow.Identifier), nil
}

func (h *Headers) IndexByChunkID(headerID, chunkID flow.Identifier) error {
	return operation.RetryOnConflictTx(h.db, transaction.Update, h.chunkIDCache.PutTx(chunkID, headerID))
}

func (h *Headers) BatchIndexByChunkID(headerID, chunkID flow.Identifier, batch storage.BatchStorage) error {
	writeBatch := batch.GetWriter()
	return operation.BatchIndexBlockByChunkID(headerID, chunkID)(writeBatch)
}
