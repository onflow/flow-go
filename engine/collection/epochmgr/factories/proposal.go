package factories

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine/collection/proposal"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/buffer"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/state/cluster"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

type ProposalEngineFactory struct {
	log            zerolog.Logger
	me             module.Local
	net            module.Network
	colMetrics     module.CollectionMetrics
	engMetrics     module.EngineMetrics
	mempoolMetrics module.MempoolMetrics
	protoState     protocol.State
	pool           mempool.Transactions // TODO make this cluster-specific
	transactions   storage.Transactions
}

// NewFactory returns a new collection proposal engine factory.
func NewProposalEngineFactory(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	colMetrics module.CollectionMetrics,
	engMetrics module.EngineMetrics,
	mempoolMetrics module.MempoolMetrics,
	protoState protocol.State,
	pool mempool.Transactions,
	transactions storage.Transactions,
) (*ProposalEngineFactory, error) {

	factory := &ProposalEngineFactory{
		log:            log,
		me:             me,
		net:            net,
		colMetrics:     colMetrics,
		engMetrics:     engMetrics,
		mempoolMetrics: mempoolMetrics,
		protoState:     protoState,
		pool:           pool,
		transactions:   transactions,
	}
	return factory, nil
}

func (f *ProposalEngineFactory) Create(clusterState cluster.State, headers storage.Headers, payloads storage.ClusterPayloads) (*proposal.Engine, error) {

	cache := buffer.NewPendingClusterBlocks()
	engine, err := proposal.New(
		f.log,
		f.net,
		f.me,
		f.colMetrics,
		f.engMetrics,
		f.mempoolMetrics,
		f.protoState,
		clusterState,
		f.pool,
		f.transactions,
		headers,
		payloads,
		cache,
	)
	return engine, err
}
