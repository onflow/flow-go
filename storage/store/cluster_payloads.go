package store

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/procedure"
)

// ClusterPayloads implements storage of block payloads for collection node
// cluster consensus.
type ClusterPayloads struct {
	db    storage.DB
	cache *Cache[flow.Identifier, *cluster.Payload]
}

func NewClusterPayloads(cacheMetrics module.CacheMetrics, db storage.DB) *ClusterPayloads {

	storeWithLock := func(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, payload *cluster.Payload) error {
		return procedure.InsertClusterPayload(lctx, rw, blockID, payload)
	}

	retrieve := func(r storage.Reader, blockID flow.Identifier) (*cluster.Payload, error) {
		var payload cluster.Payload
		err := procedure.RetrieveClusterPayload(r, blockID, &payload)
		return &payload, err
	}

	cp := &ClusterPayloads{
		db: db,
		cache: newCache[flow.Identifier, *cluster.Payload](cacheMetrics, metrics.ResourceClusterPayload,
			withLimit[flow.Identifier, *cluster.Payload](flow.DefaultTransactionExpiry*4),
			withStoreWithLock(storeWithLock),
			withRetrieve(retrieve)),
	}

	return cp
}

func (cp *ClusterPayloads) storeTx(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, payload *cluster.Payload) error {
	return cp.cache.PutWithLockTx(lctx, rw, blockID, payload)
}

func (cp *ClusterPayloads) retrieveTx(r storage.Reader, blockID flow.Identifier) (*cluster.Payload, error) {
	val, err := cp.cache.Get(r, blockID)
	if err != nil {
		return nil, err
	}
	return val, nil
}


func (cp *ClusterPayloads) ByBlockID(blockID flow.Identifier) (*cluster.Payload, error) {
	return cp.retrieveTx(cp.db.Reader(), blockID)
}
