package factories

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	clusterkv "github.com/dapperlabs/flow-go/state/cluster/badger"
	bstorage "github.com/dapperlabs/flow-go/storage/badger"
)

type ClusterStateFactory struct {
	db      *badger.DB
	metrics module.CacheMetrics
}

func NewClusterStateFactory(
	db *badger.DB,
	metrics module.CacheMetrics,
) (*ClusterStateFactory, error) {
	factory := &ClusterStateFactory{
		db:      db,
		metrics: metrics,
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
		clusterID,
		headers,
		payloads,
	)
	return clusterState, headers, payloads, blocks, err
}
