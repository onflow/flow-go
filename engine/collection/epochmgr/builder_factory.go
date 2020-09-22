package epochmgr

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/module"
	builder "github.com/dapperlabs/flow-go/module/builder/collection"
	finalizer "github.com/dapperlabs/flow-go/module/finalizer/collection"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/storage"
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

	build := builder.NewBuilder(
		f.db,
		f.mainChainHeaders,
		clusterHeaders,
		clusterPayloads,
		pool,
		f.trace,
		f.opts...,
	)

	final := finalizer.NewFinalizer(
		f.db,
		pool,
		f.pusher,
		f.metrics,
	)

	return build, final, nil
}
