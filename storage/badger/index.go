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
	index *Cache
}

func NewIndex(collector module.CacheMetrics, db *badger.DB) *Index {

	storeIndex := func(blockID flow.Identifier, index interface{}) error {
		idx := index.(flow.Index)
		err := operation.RetryOnConflict(db.Update, operation.IndexPayloadIdentities(blockID, idx.NodeIDs))
		if err != nil {
			return fmt.Errorf("could not store identity index: %w", err)
		}
		err = operation.RetryOnConflict(db.Update, operation.IndexPayloadGuarantees(blockID, idx.CollectionIDs))
		if err != nil {
			return fmt.Errorf("could not store guarantee index: %w", err)
		}
		err = operation.RetryOnConflict(db.Update, operation.IndexPayloadSeals(blockID, idx.SealIDs))
		if err != nil {
			return fmt.Errorf("could not store seal index: %w", err)
		}
		return nil
	}

	retrieveIndex := func(blockID flow.Identifier) (interface{}, error) {
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

	p := &Index{
		db: db,
		index: newCache(collector,
			withLimit(flow.DefaultTransactionExpiry),
			withStore(storeIndex),
			withRetrieve(retrieveIndex),
			withResource(metrics.ResourceIndex),
		),
	}

	return p
}

func (i *Index) Store(blockID flow.Identifier, index flow.Index) error {
	return i.index.Put(blockID, index)
}

func (i *Index) ByBlockID(blockID flow.Identifier) (flow.Index, error) {
	index, err := i.index.Get(blockID)
	if err != nil {
		return flow.Index{}, err
	}
	return index.(flow.Index), nil
}
