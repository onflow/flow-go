package factories

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	builder "github.com/onflow/flow-go/module/builder/collection"
	finalizer "github.com/onflow/flow-go/module/finalizer/collection"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/network"
	clusterstate "github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type BuilderFactory struct {
	db               *badger.DB
	protoState       protocol.State
	mainChainHeaders storage.Headers
	trace            module.Tracer
	opts             []builder.Opt
	metrics          module.CollectionMetrics
	pusher           network.Engine // engine for pushing finalized collection to consensus committee
	log              zerolog.Logger
}

func NewBuilderFactory(
	db *badger.DB,
	protoState protocol.State,
	mainChainHeaders storage.Headers,
	trace module.Tracer,
	metrics module.CollectionMetrics,
	pusher network.Engine,
	log zerolog.Logger,
	opts ...builder.Opt,
) (*BuilderFactory, error) {

	factory := &BuilderFactory{
		db:               db,
		protoState:       protoState,
		mainChainHeaders: mainChainHeaders,
		trace:            trace,
		metrics:          metrics,
		pusher:           pusher,
		log:              log,
		opts:             opts,
	}
	return factory, nil
}

func (f *BuilderFactory) Create(
	clusterState clusterstate.State,
	clusterHeaders storage.Headers,
	clusterPayloads storage.ClusterPayloads,
	pool mempool.Transactions,
	epoch uint64,
) (module.Builder, *finalizer.Finalizer, error) {

	build, err := builder.NewBuilder(
		f.db,
		f.trace,
		f.protoState,
		clusterState,
		f.mainChainHeaders,
		clusterHeaders,
		clusterPayloads,
		pool,
		f.log,
		epoch,
		f.opts...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create builder: %w", err)
	}

	final := finalizer.NewFinalizer(
		f.db,
		pool,
		f.pusher,
		f.metrics,
	)

	return build, final, nil
}
