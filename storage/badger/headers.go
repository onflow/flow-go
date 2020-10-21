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
)

// Headers implements a simple read-only header storage around a badger DB.
type Headers struct {
	db    *badger.DB
	cache *Cache
}

func NewHeaders(collector module.CacheMetrics, db *badger.DB) *Headers {

	store := func(key interface{}, val interface{}) func(tx *badger.Txn) error {
		blockID := key.(flow.Identifier)
		header := val.(*flow.Header)
		return operation.InsertHeader(blockID, header)
	}

	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		blockID := key.(flow.Identifier)
		var header flow.Header
		return func(tx *badger.Txn) (interface{}, error) {
			err := db.View(operation.RetrieveHeader(blockID, &header))
			return &header, err
		}
	}

	h := &Headers{
		db: db,
		cache: newCache(collector,
			withLimit(4*flow.DefaultTransactionExpiry),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceHeader)),
	}

	return h
}

func (h *Headers) storeTx(header *flow.Header) func(*badger.Txn) error {
	return h.cache.Put(header.ID(), header)
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

func (h *Headers) Store(header *flow.Header) error {
	return operation.RetryOnConflict(h.db.Update, h.storeTx(header))
}

func (h *Headers) ByBlockID(blockID flow.Identifier) (*flow.Header, error) {
	tx := h.db.NewTransaction(false)
	defer tx.Discard()
	return h.retrieveTx(blockID)(tx)
}

func (h *Headers) ByHeight(height uint64) (*flow.Header, error) {
	var blockID flow.Identifier
	err := h.db.View(operation.LookupBlockHeight(height, &blockID))
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return h.ByBlockID(blockID)
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

// GetLastSealed returns the header of the heighest sealed block
func (b *Blocks) GetLastSealed() (*flow.Header, error) {
	var sealedHeight uint64
	err := h.db.View(operation.RetrieveSealedHeight(&sealedHeight))
	if err != nil {
		return nil, fmt.Errorf("could no retrieve sealed height: %w", err)
	}
	return h.ByHeight(sealedHeight)
}
