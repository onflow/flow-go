package factories

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	clusterkv "github.com/onflow/flow-go/state/cluster/badger"
	bstorage "github.com/onflow/flow-go/storage/badger"
)

type ClusterStateFactory struct {
	db      *badger.DB
	metrics module.CacheMetrics
	tracer  module.Tracer
}

func NewClusterStateFactory(
	db *badger.DB,
	metrics module.CacheMetrics,
	tracer module.Tracer,
) (*ClusterStateFactory, error) {
	factory := &ClusterStateFactory{
		db:      db,
		metrics: metrics,
		tracer:  tracer,
	}
	return factory, nil
}

func (f *ClusterStateFactory) Create(clusterID flow.ChainID) (
	*clusterkv.State,
	*bstorage.Headers,
	*bstorage.ClusterPayloads,
	*bstorage.ClusterBlocks,
	error,
) {

	headers := bstorage.NewHeaders(f.metrics, f.db)
	payloads := bstorage.NewClusterPayloads(f.metrics, f.db)
	blocks := bstorage.NewClusterBlocks(f.db, clusterID, headers, payloads)

	clusterState, err := clusterkv.NewState(
		f.db,
		f.tracer,
		clusterID,
		headers,
		payloads,
	)
	return clusterState, headers, payloads, blocks, err
}
