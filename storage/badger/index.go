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

// Index implements a simple read-only payload storage around a badger DB.
type Index struct {
	db    *badger.DB
	cache *Cache
}

func NewIndex(collector module.CacheMetrics, db *badger.DB) *Index {

	p := &Index{db: db}

	store := func(blockID flow.Identifier, index interface{}) error {
		return operation.RetryOnConflict(db.Update, p.storeTx(blockID, index.(flow.Index)))
	}

	retrieve := func(blockID flow.Identifier) (interface{}, error) {
		var nodeIDs []flow.Identifier
		err := db.View(operation.LookupPayloadIdentities(blockID, &nodeIDs))
		if err != nil {
			return nil, fmt.Errorf("could not retrieve identity index: %w", err)
		}
		var collIDs []flow.Identifier
		err = db.View(operation.LookupPayloadGuarantees(blockID, &collIDs))
		if err != nil {
			return nil, fmt.Errorf("could not retrieve guarantee index: %w", err)
		}
		var sealIDs []flow.Identifier
		err = db.View(operation.LookupPayloadSeals(blockID, &sealIDs))
		if err != nil {
			return nil, fmt.Errorf("could not retrieve seal index: %w", err)
		}
		idx := flow.Index{
			NodeIDs:       nodeIDs,
			CollectionIDs: collIDs,
			SealIDs:       sealIDs,
		}
		return idx, nil
	}

	p.cache = newCache(collector,
		withLimit(flow.DefaultTransactionExpiry+100),
		withStore(store),
		withRetrieve(retrieve),
		withResource(metrics.ResourceIndex))

	return p
}

func (i *Index) Store(blockID flow.Identifier, index flow.Index) error {
	return i.cache.Put(blockID, index)
}

func (i *Index) storeTx(blockID flow.Identifier, index flow.Index) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		err := operation.IndexPayloadIdentities(blockID, index.NodeIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not store identity index: %w", err)
		}
		err = operation.IndexPayloadGuarantees(blockID, index.CollectionIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not store guarantee index: %w", err)
		}
		err = operation.IndexPayloadSeals(blockID, index.SealIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not store seal index: %w", err)
		}
		return nil
	}
}

func (i *Index) ByBlockID(blockID flow.Identifier) (flow.Index, error) {
	index, err := i.cache.Get(blockID)
	if err != nil {
		return flow.Index{}, err
	}
	return index.(flow.Index), nil
}
