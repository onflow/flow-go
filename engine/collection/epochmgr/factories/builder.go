package factories

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module"
	builder "github.com/onflow/flow-go/module/builder/collection"
	finalizer "github.com/onflow/flow-go/module/finalizer/collection"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/storage"
)

type BuilderFactory struct {
	db               *badger.DB
	mainChainHeaders storage.Headers
	trace            module.Tracer
	opts             []builder.Opt
	metrics          module.CollectionMetrics
	pusher           network.Engine // engine for pushing finalized collection to consensus committee
}

func NewBuilderFactory(
	db *badger.DB,
	mainChainHeaders storage.Headers,
	trace module.Tracer,
	metrics module.CollectionMetrics,
	pusher network.Engine,
	opts ...builder.Opt,
) (*BuilderFactory, error) {

	factory := &BuilderFactory{
		db:               db,
		mainChainHeaders: mainChainHeaders,
		trace:            trace,
		metrics:          metrics,
		pusher:           pusher,
		opts:             opts,
	}
	return factory, nil
}

func (f *BuilderFactory) Create(
	clusterHeaders storage.Headers,
	clusterPayloads storage.ClusterPayloads,
	pool mempool.Transactions,
) (module.Builder, *finalizer.Finalizer, error) {

	build, err := builder.NewBuilder(
		f.db,
		f.trace,
		f.mainChainHeaders,
		clusterHeaders,
		clusterPayloads,
		pool,
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
