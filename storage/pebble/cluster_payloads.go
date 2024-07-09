package pebble

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/storage/pebble/procedure"
)

// ClusterPayloads implements storage of block payloads for collection node
// cluster consensus.
type ClusterPayloads struct {
	db    *pebble.DB
	cache *Cache[flow.Identifier, *cluster.Payload]
}

func NewClusterPayloads(cacheMetrics module.CacheMetrics, db *pebble.DB) *ClusterPayloads {

	retrieve := func(blockID flow.Identifier) func(tx pebble.Reader) (*cluster.Payload, error) {
		var payload cluster.Payload
		return func(tx pebble.Reader) (*cluster.Payload, error) {
			err := procedure.RetrieveClusterPayload(blockID, &payload)(tx)
			return &payload, err
		}
	}

	cp := &ClusterPayloads{
		db: db,
		cache: newCache[flow.Identifier, *cluster.Payload](cacheMetrics, metrics.ResourceClusterPayload,
			withLimit[flow.Identifier, *cluster.Payload](flow.DefaultTransactionExpiry*4),
			withRetrieve(retrieve)),
	}

	return cp
}

func (cp *ClusterPayloads) storeTx(blockID flow.Identifier, payload *cluster.Payload) func(storage.PebbleReaderBatchWriter) error {
	return func(tx storage.PebbleReaderBatchWriter) error {
		_, w := tx.ReaderWriter()

		tx.AddCallback(func() {
			cp.cache.Insert(blockID, payload)
		})

		return procedure.InsertClusterPayload(blockID, payload)(w)
	}
}
func (cp *ClusterPayloads) retrieveTx(blockID flow.Identifier) func(pebble.Reader) (*cluster.Payload, error) {
	return func(tx pebble.Reader) (*cluster.Payload, error) {
		val, err := cp.cache.Get(blockID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

func (cp *ClusterPayloads) Store(blockID flow.Identifier, payload *cluster.Payload) error {
	return operation.WithReaderBatchWriter(cp.db, cp.storeTx(blockID, payload))
}

func (cp *ClusterPayloads) ByBlockID(blockID flow.Identifier) (*cluster.Payload, error) {
	return cp.retrieveTx(blockID)(cp.db)
}
