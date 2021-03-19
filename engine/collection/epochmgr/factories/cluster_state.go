package factories

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

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

func (f *ClusterStateFactory) Create(stateRoot *clusterkv.StateRoot) (
	*clusterkv.MutableState,
	*bstorage.Headers,
	*bstorage.ClusterPayloads,
	*bstorage.ClusterBlocks,
	error,
) {

	headers := bstorage.NewHeaders(f.metrics, f.db)
	payloads := bstorage.NewClusterPayloads(f.metrics, f.db)
	blocks := bstorage.NewClusterBlocks(f.db, stateRoot.ClusterID(), headers, payloads)

	isBootStrapped, err := clusterkv.IsBootstrapped(f.db, stateRoot.ClusterID())
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not check cluster state db: %w", err)
	}
	var clusterState *clusterkv.State
	if isBootStrapped {
		clusterState, err = clusterkv.OpenState(f.db, f.tracer, headers, payloads, stateRoot.ClusterID())
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("could not open cluster state: %w", err)
		}
	} else {
		clusterState, err = clusterkv.Bootstrap(f.db, stateRoot)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("could not bootstrap cluster state: %w", err)
		}
	}

	mutableState, err := clusterkv.NewMutableState(clusterState, f.tracer, headers, payloads)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could create mutable cluster state: %w", err)
	}
	return mutableState, headers, payloads, blocks, err
}
