package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// ClusterPayloads implements storage of block payloads for collection node
// cluster consensus.
type ClusterPayloads struct {
	db    *badger.DB
	cache *Cache
}

func NewClusterPayloads(cacheMetrics module.CacheMetrics, db *badger.DB) *ClusterPayloads {

	cp := &ClusterPayloads{db: db}

	store := func(blockID flow.Identifier, payload interface{}) error {
		return operation.RetryOnConflict(db.Update, cp.storeTx(blockID, payload.(*cluster.Payload)))
	}

	retrieve := func(blockID flow.Identifier) (interface{}, error) {
		var payload cluster.Payload
		err := db.View(procedure.RetrieveClusterPayload(blockID, &payload))
		return &payload, err
	}

	cp.cache = newCache(cacheMetrics,
		withLimit(flow.DefaultTransactionExpiry+100),
		withStore(store),
		withRetrieve(retrieve),
		withResource(metrics.ResourceIndex))

	return cp
}

func (cp *ClusterPayloads) Store(blockID flow.Identifier, payload *cluster.Payload) error {
	return cp.cache.Put(blockID, payload)
}

func (cp *ClusterPayloads) storeTx(blockID flow.Identifier, payload *cluster.Payload) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		return procedure.InsertClusterPayload(blockID, payload)(tx)
	}
}

func (cp *ClusterPayloads) ByBlockID(blockID flow.Identifier) (*cluster.Payload, error) {
	payload, err := cp.cache.Get(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve cluster payload: %w", err)
	}
	return payload.(*cluster.Payload), nil
}
