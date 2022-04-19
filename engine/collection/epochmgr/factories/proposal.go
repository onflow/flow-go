package factories

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/engine/collection/compliance"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	modulecompliance "github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type ProposalEngineFactory struct {
	log            zerolog.Logger
	me             module.Local
	net            network.Network
	colMetrics     module.CollectionMetrics
	engMetrics     module.EngineMetrics
	mempoolMetrics module.MempoolMetrics
	protoState     protocol.State
	transactions   storage.Transactions
	complianceOpts []modulecompliance.Opt
}

// NewFactory returns a new collection proposal engine factory.
func NewProposalEngineFactory(
	log zerolog.Logger,
	net network.Network,
	me module.Local,
	colMetrics module.CollectionMetrics,
	engMetrics module.EngineMetrics,
	mempoolMetrics module.MempoolMetrics,
	protoState protocol.State,
	transactions storage.Transactions,
	complianceOpts ...modulecompliance.Opt,
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
		complianceOpts: complianceOpts,
	}
	return factory, nil
}

func (f *ProposalEngineFactory) Create(
	clusterState cluster.MutableState,
	headers storage.Headers,
	payloads storage.ClusterPayloads,
	voteAggregator hotstuff.VoteAggregator,
) (*compliance.Engine, error) {

	cache := buffer.NewPendingClusterBlocks()
	core, err := compliance.NewCore(
		f.log,
		f.engMetrics,
		f.mempoolMetrics,
		f.colMetrics,
		headers,
		clusterState,
		cache,
		voteAggregator,
		f.complianceOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("could create cluster compliance core: %w", err)
	}

	engine, err := compliance.NewEngine(
		f.log,
		f.net,
		f.me,
		f.protoState,
		payloads,
		core,
	)
	return engine, err
}
