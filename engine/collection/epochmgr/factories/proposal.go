package factories

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/collection/proposal"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type ProposalEngineFactory struct {
	log            zerolog.Logger
	me             module.Local
	net            module.Network
	colMetrics     module.CollectionMetrics
	engMetrics     module.EngineMetrics
	mempoolMetrics module.MempoolMetrics
	protoState     protocol.State
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
		transactions:   transactions,
	}
	return factory, nil
}

func (f *ProposalEngineFactory) Create(clusterState cluster.MutableState, headers storage.Headers, payloads storage.ClusterPayloads) (*proposal.Engine, error) {

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
		f.transactions,
		headers,
		payloads,
		cache,
	)
	return engine, err
}
