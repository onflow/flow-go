package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// ClusterPayloads implements storage of block payloads for collection node
// cluster consensus.
type ClusterPayloads struct {
	db    *badger.DB
	cache *Cache[flow.Identifier, *cluster.Payload]
}

func NewClusterPayloads(cacheMetrics module.CacheMetrics, db *badger.DB) *ClusterPayloads {

	store := func(blockID flow.Identifier, payload *cluster.Payload) func(*transaction.Tx) error {
		return transaction.WithTx(procedure.InsertClusterPayload(blockID, payload))
	}

	retrieve := func(blockID flow.Identifier) func(tx *badger.Txn) (*cluster.Payload, error) {
		var payload cluster.Payload
		return func(tx *badger.Txn) (*cluster.Payload, error) {
			err := procedure.RetrieveClusterPayload(blockID, &payload)(tx)
			return &payload, err
		}
	}

	cp := &ClusterPayloads{
		db: db,
		cache: newCache[flow.Identifier, *cluster.Payload](cacheMetrics, metrics.ResourceClusterPayload,
			withLimit[flow.Identifier, *cluster.Payload](flow.DefaultTransactionExpiry*4),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return cp
}

func (cp *ClusterPayloads) storeTx(blockID flow.Identifier, payload *cluster.Payload) func(*transaction.Tx) error {
	return cp.cache.PutTx(blockID, payload)
}
func (cp *ClusterPayloads) retrieveTx(blockID flow.Identifier) func(*badger.Txn) (*cluster.Payload, error) {
	return func(tx *badger.Txn) (*cluster.Payload, error) {
		val, err := cp.cache.Get(blockID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

func (cp *ClusterPayloads) Store(blockID flow.Identifier, payload *cluster.Payload) error {
	return operation.RetryOnConflictTx(cp.db, transaction.Update, cp.storeTx(blockID, payload))
}

func (cp *ClusterPayloads) ByBlockID(blockID flow.Identifier) (*cluster.Payload, error) {
	tx := cp.db.NewTransaction(false)
	defer tx.Discard()
	return cp.retrieveTx(blockID)(tx)
}
