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

	store := func(blockID flow.Identifier, payload interface{}) error {
		return operation.RetryOnConflict(db.Update, procedure.InsertClusterPayload(blockID, payload.(*cluster.Payload)))
	}

	retrieve := func(blockID flow.Identifier) (interface{}, error) {
		var payload cluster.Payload
		err := db.View(procedure.RetrieveClusterPayload(blockID, &payload))
		return &payload, err
	}

	cp := &ClusterPayloads{
		db: db,
		cache: newCache(cacheMetrics,
			withLimit(flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceClusterPayload)),
	}

	return cp
}

func (cp *ClusterPayloads) Store(blockID flow.Identifier, payload *cluster.Payload) error {
	return cp.cache.Put(blockID, payload)
}

func (cp *ClusterPayloads) ByBlockID(blockID flow.Identifier) (*cluster.Payload, error) {
	payload, err := cp.cache.Get(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve cluster payload: %w", err)
	}
	return payload.(*cluster.Payload), nil
}
