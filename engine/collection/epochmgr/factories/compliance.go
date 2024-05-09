package factories

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/engine/collection/compliance"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	"github.com/onflow/flow-go/module/chainsync"
	modulecompliance "github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type ComplianceEngineFactory struct {
	log            zerolog.Logger
	me             module.Local
	net            network.EngineRegistry
	colMetrics     module.CollectionMetrics
	engMetrics     module.EngineMetrics
	mempoolMetrics module.MempoolMetrics
	protoState     protocol.State
	transactions   storage.Transactions
	config         modulecompliance.Config
}

// NewComplianceEngineFactory returns a new collection compliance engine factory.
func NewComplianceEngineFactory(
	log zerolog.Logger,
	net network.EngineRegistry,
	me module.Local,
	colMetrics module.CollectionMetrics,
	engMetrics module.EngineMetrics,
	mempoolMetrics module.MempoolMetrics,
	protoState protocol.State,
	transactions storage.Transactions,
	config modulecompliance.Config,
) (*ComplianceEngineFactory, error) {

	factory := &ComplianceEngineFactory{
		log:            log,
		me:             me,
		net:            net,
		colMetrics:     colMetrics,
		engMetrics:     engMetrics,
		mempoolMetrics: mempoolMetrics,
		protoState:     protoState,
		transactions:   transactions,
		config:         config,
	}
	return factory, nil
}

func (f *ComplianceEngineFactory) Create(
	hotstuffMetrics module.HotstuffMetrics,
	notifier hotstuff.ProposalViolationConsumer,
	clusterState cluster.MutableState,
	headers storage.Headers,
	payloads storage.ClusterPayloads,
	syncCore *chainsync.Core,
	hot module.HotStuff,
	voteAggregator hotstuff.VoteAggregator,
	timeoutAggregator hotstuff.TimeoutAggregator,
	validator hotstuff.Validator,
) (*compliance.Engine, error) {

	cache := buffer.NewPendingClusterBlocks()
	core, err := compliance.NewCore(
		f.log,
		f.engMetrics,
		f.mempoolMetrics,
		hotstuffMetrics,
		f.colMetrics,
		notifier,
		headers,
		clusterState,
		cache,
		syncCore,
		validator,
		hot,
		voteAggregator,
		timeoutAggregator,
		f.config,
	)
	if err != nil {
		return nil, fmt.Errorf("could create cluster compliance core: %w", err)
	}

	engine, err := compliance.NewEngine(
		f.log,
		f.me,
		f.protoState,
		payloads,
		core,
	)
	return engine, err
}
