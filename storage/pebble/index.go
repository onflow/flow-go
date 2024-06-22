// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package pebble

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/storage/pebble/procedure"
)

// Index implements a simple read-only payload storage around a pebble DB.
type Index struct {
	db    *pebble.DB
	cache *Cache[flow.Identifier, *flow.Index]
}

func NewIndex(collector module.CacheMetrics, db *pebble.DB) *Index {

	store := func(blockID flow.Identifier, index *flow.Index) func(operation.PebbleReaderWriter) error {
		return operation.OnlyWrite(procedure.InsertIndex(blockID, index))
	}

	retrieve := func(blockID flow.Identifier) func(tx pebble.Reader) (*flow.Index, error) {
		var index flow.Index
		return func(tx pebble.Reader) (*flow.Index, error) {
			err := procedure.RetrieveIndex(blockID, &index)(tx)
			return &index, err
		}
	}

	p := &Index{
		db: db,
		cache: newCache[flow.Identifier, *flow.Index](collector, metrics.ResourceIndex,
			withLimit[flow.Identifier, *flow.Index](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return p
}

func (i *Index) storeTx(blockID flow.Identifier, index *flow.Index) func(operation.PebbleReaderWriter) error {
	return i.cache.PutTx(blockID, index)
}

func (i *Index) retrieveTx(blockID flow.Identifier) func(pebble.Reader) (*flow.Index, error) {
	return func(tx pebble.Reader) (*flow.Index, error) {
		val, err := i.cache.Get(blockID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

func (i *Index) Store(blockID flow.Identifier, index *flow.Index) error {
	return i.storeTx(blockID, index)(i.db)
}

func (i *Index) ByBlockID(blockID flow.Identifier) (*flow.Index, error) {
	return i.retrieveTx(blockID)(i.db)
}
