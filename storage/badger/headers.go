// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Headers implements a simple read-only header storage around a badger DB.
type Headers struct {
	db    *badger.DB
	cache *Cache
}

func NewHeaders(collector module.CacheMetrics, db *badger.DB) *Headers {

	store := func(blockID flow.Identifier, v interface{}) func(tx *badger.Txn) error {
		header := v.(*flow.Header)
		return operation.InsertHeader(blockID, header)
	}

	retrieve := func(blockID flow.Identifier) func(tx *badger.Txn) (interface{}, error) {
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
		v, err := h.cache.Get(blockID)(tx)
		return v.(*flow.Header), err
	}
}

func (h *Headers) Store(header *flow.Header) error {
	return operation.RetryOnConflict(h.db.Update, h.storeTx(header))
}

func (h *Headers) ByBlockID(blockID flow.Identifier) (*flow.Header, error) {
	return h.retrieveTx(blockID)(h.db.NewTransaction(false))
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
