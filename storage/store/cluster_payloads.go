package store

import (
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

	store := func(rw storage.ReaderBatchWriter, blockID flow.Identifier, payload *cluster.Payload) error {
		return procedure.InsertClusterPayload(rw, blockID, payload)
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
			withStore(store),
			withRetrieve(retrieve)),
	}

	return cp
}

func (cp *ClusterPayloads) storeTx(rw storage.ReaderBatchWriter, blockID flow.Identifier, payload *cluster.Payload) error {
	return cp.cache.PutTx(rw, blockID, payload)
}

func (cp *ClusterPayloads) retrieveTx(r storage.Reader, blockID flow.Identifier) (*cluster.Payload, error) {
	val, err := cp.cache.Get(r, blockID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (cp *ClusterPayloads) Store(blockID flow.Identifier, payload *cluster.Payload) error {
	return cp.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return cp.storeTx(rw, blockID, payload)
	})
}

func (cp *ClusterPayloads) ByBlockID(blockID flow.Identifier) (*cluster.Payload, error) {
	return cp.retrieveTx(cp.db.Reader(), blockID)
}
