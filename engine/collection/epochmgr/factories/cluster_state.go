package factories

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	clusterkv "github.com/onflow/flow-go/state/cluster/badger"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

type ClusterStateFactory struct {
	db          storage.DB
	lockManager lockctx.Manager
	metrics     module.CacheMetrics
	tracer      module.Tracer
}

func NewClusterStateFactory(
	db storage.DB,
	lockManager lockctx.Manager,
	metrics module.CacheMetrics,
	tracer module.Tracer,
) (*ClusterStateFactory, error) {
	factory := &ClusterStateFactory{
		db:          db,
		lockManager: lockManager,
		metrics:     metrics,
		tracer:      tracer,
	}
	return factory, nil
}

func (f *ClusterStateFactory) Create(stateRoot *clusterkv.StateRoot, chainID flow.ChainID) (
	*clusterkv.MutableState,
	*store.Headers,
	storage.ClusterPayloads,
	storage.ClusterBlocks,
	*store.Headers,
	error,
) {

	headers := store.NewClusterHeaders(f.metrics, f.db, stateRoot.ClusterID())
	payloads := store.NewClusterPayloads(f.metrics, f.db)
	blocks := store.NewClusterBlocks(f.db, stateRoot.ClusterID(), headers, payloads)
	consensusHeaders := store.NewHeaders(f.metrics, f.db, chainID) // for reference blocks

	isBootStrapped, err := clusterkv.IsBootstrapped(f.db, stateRoot.ClusterID())
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("could not check cluster state db: %w", err)
	}
	var clusterState *clusterkv.State
	if isBootStrapped {
		clusterState, err = clusterkv.OpenState(f.db, f.tracer, headers, payloads, stateRoot.ClusterID(), stateRoot.EpochCounter())
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("could not open cluster state: %w", err)
		}
	} else {
		clusterState, err = clusterkv.Bootstrap(f.db, f.lockManager, stateRoot)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("could not bootstrap cluster state: %w", err)
		}
	}

	mutableState, err := clusterkv.NewMutableState(clusterState, f.lockManager, f.tracer, headers, payloads, consensusHeaders)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("could create mutable cluster state: %w", err)
	}
	return mutableState, headers, payloads, blocks, consensusHeaders, err
}
