package store

import (
	"fmt"

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

var _ storage.ClusterPayloads = (*ClusterPayloads)(nil)

func NewClusterPayloads(cacheMetrics module.CacheMetrics, db storage.DB) *ClusterPayloads {
	retrieve := func(r storage.Reader, blockID flow.Identifier) (*cluster.Payload, error) {
		var payload cluster.Payload
		err := procedure.RetrieveClusterPayload(r, blockID, &payload)
		return &payload, err
	}

	cp := &ClusterPayloads{
		db: db,
		cache: newCache[flow.Identifier, *cluster.Payload](cacheMetrics, metrics.ResourceClusterPayload,
			withLimit[flow.Identifier, *cluster.Payload](flow.DefaultTransactionExpiry*4),
			withRetrieve(retrieve)),
	}
	return cp
}

func (cp *ClusterPayloads) ByBlockID(blockID flow.Identifier) (*cluster.Payload, error) {
	val, err := cp.cache.Get(cp.db.Reader(), blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve cluster block payload: %w", err)
	}
	return val, nil
}
