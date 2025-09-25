package factories

import (
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/collection"
	"github.com/onflow/flow-go/module"
	builder "github.com/onflow/flow-go/module/builder/collection"
	finalizer "github.com/onflow/flow-go/module/finalizer/collection"
	"github.com/onflow/flow-go/module/mempool"
	clusterstate "github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type BuilderFactory struct {
	db               storage.DB
	protoState       protocol.State
	lockManager      lockctx.Manager
	mainChainHeaders storage.Headers
	trace            module.Tracer
	opts             []builder.Opt
	metrics          module.CollectionMetrics
	pusher           collection.GuaranteedCollectionPublisher // engine for pushing finalized collection to consensus committee
	configGetter     module.ReadonlySealingLagRateLimiterConfig
	log              zerolog.Logger
}

func NewBuilderFactory(
	db storage.DB,
	protoState protocol.State,
	lockManager lockctx.Manager,
	mainChainHeaders storage.Headers,
	trace module.Tracer,
	metrics module.CollectionMetrics,
	pusher collection.GuaranteedCollectionPublisher,
	log zerolog.Logger,
	configGetter module.ReadonlySealingLagRateLimiterConfig,
	opts ...builder.Opt,
) (*BuilderFactory, error) {

	factory := &BuilderFactory{
		db:               db,
		protoState:       protoState,
		lockManager:      lockManager,
		mainChainHeaders: mainChainHeaders,
		trace:            trace,
		metrics:          metrics,
		pusher:           pusher,
		log:              log,
		configGetter:     configGetter,
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
		f.lockManager,
		f.metrics,
		f.protoState,
		clusterState,
		f.mainChainHeaders,
		clusterHeaders,
		clusterPayloads,
		pool,
		f.log,
		epoch,
		f.configGetter,
		f.opts...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create builder: %w", err)
	}

	final := finalizer.NewFinalizer(
		f.db,
		f.lockManager,
		pool,
		f.pusher,
		f.metrics,
	)

	return build, final, nil
}
