package badger

import (
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

	store := func(blockID flow.Identifier, v interface{}) func(tx *badger.Txn) error {
		payload := v.(*cluster.Payload)
		return procedure.InsertClusterPayload(blockID, payload)
	}

	retrieve := func(blockID flow.Identifier) func(tx *badger.Txn) (interface{}, error) {
		var payload cluster.Payload
		return func(tx *badger.Txn) (interface{}, error) {
			err := db.View(procedure.RetrieveClusterPayload(blockID, &payload))
			return &payload, err
		}
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

func (cp *ClusterPayloads) storeTx(blockID flow.Identifier, payload *cluster.Payload) func(*badger.Txn) error {
	return cp.cache.Put(blockID, payload)
}
func (cp *ClusterPayloads) retrieveTx(blockID flow.Identifier) func(*badger.Txn) (*cluster.Payload, error) {
	return func(tx *badger.Txn) (*cluster.Payload, error) {
		v, err := cp.cache.Get(blockID)(tx)
		return v.(*cluster.Payload), err
	}
}

func (cp *ClusterPayloads) Store(blockID flow.Identifier, payload *cluster.Payload) error {
	return operation.RetryOnConflict(cp.db.Update, cp.storeTx(blockID, payload))
}

func (cp *ClusterPayloads) ByBlockID(blockID flow.Identifier) (*cluster.Payload, error) {
	tx := cp.db.NewTransaction(false)
	defer tx.Discard()
	return cp.retrieveTx(blockID)(tx)
}
