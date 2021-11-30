package factories

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/blockproducer"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	recovery "github.com/onflow/flow-go/consensus/recovery/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	hotmetrics "github.com/onflow/flow-go/module/metrics/hotstuff"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type HotStuffMetricsFunc func(chainID flow.ChainID) module.HotstuffMetrics

type HotStuffFactory struct {
	log           zerolog.Logger
	me            module.Local
	aggregator    module.AggregatingSigner
	db            *badger.DB
	protoState    protocol.State
	createMetrics HotStuffMetricsFunc
	opts          []consensus.Option
}

func NewHotStuffFactory(
	log zerolog.Logger,
	me module.Local,
	aggregator module.AggregatingSigner,
	db *badger.DB,
	protoState protocol.State,
	createMetrics HotStuffMetricsFunc,
	opts ...consensus.Option,
) (*HotStuffFactory, error) {

	factory := &HotStuffFactory{
		log:           log,
		me:            me,
		aggregator:    aggregator,
		db:            db,
		protoState:    protoState,
		createMetrics: createMetrics,
		opts:          opts,
	}
	return factory, nil
}

func (f *HotStuffFactory) CreateModules(epoch protocol.Epoch,
	cluster protocol.Cluster,
	clusterState cluster.State,
	headers storage.Headers,
	payloads storage.ClusterPayloads,
	updater module.Finalizer,
) (*consensus.HotstuffModules, error) {

	// setup metrics/logging with the new chain ID
	metrics := f.createMetrics(cluster.ChainID())
	notifier := pubsub.NewDistributor()
	notifier.AddConsumer(notifications.NewLogConsumer(f.log))
	notifier.AddConsumer(hotmetrics.NewMetricsConsumer(metrics))
	notifier.AddConsumer(notifications.NewTelemetryConsumer(f.log, cluster.ChainID()))

	var (
		err       error
		committee hotstuff.Committee
	)
	committee, err = committees.NewClusterCommittee(f.protoState, payloads, cluster, epoch, f.me.NodeID())
	if err != nil {
		return nil, fmt.Errorf("could not create cluster committee: %w", err)
	}
	committee = committees.NewMetricsWrapper(committee, metrics) // wrapper for measuring time spent determining consensus committee relations

	// create a signing provider
	var signer hotstuff.SignerVerifier
	signer = verification.NewSingleSignerVerifier(committee, f.aggregator, f.me.NodeID())
	signer = verification.NewMetricsWrapper(signer, metrics) // wrapper for measuring time spent with crypto-related operations

	finalizedBlock, pendingBlocks, err := recovery.FindLatest(clusterState, headers)
	if err != nil {
		return nil, err
	}

	forks, err := consensus.NewForks(
		finalizedBlock,
		headers,
		updater,
		notifier,
		cluster.RootBlock().Header,
		cluster.RootQC(),
	)
	if err != nil {
		return nil, err
	}

	qcDistributor := pubsub.NewQCCreatedDistributor()
	validator := consensus.NewValidator(metrics, committee, forks, signer)
	voteProcessorFactory := votecollector.NewStakingVoteProcessorFactory(committee, qcDistributor.OnQcConstructedFromVotes)
	aggregator, err := consensus.NewVoteAggregator(
		f.log,
		finalizedBlock,
		pendingBlocks,
		notifier,
		forks,
		validator,
		voteProcessorFactory,
	)
	if err != nil {
		return nil, err
	}

	return &consensus.HotstuffModules{
		Forks:                forks,
		Validator:            validator,
		Notifier:             notifier,
		Committee:            committee,
		Signer:               signer,
		Persist:              persister.New(f.db, cluster.ChainID()),
		Aggregator:           aggregator,
		QCCreatedDistributor: qcDistributor,
	}, nil
}

func (f *HotStuffFactory) Create(
	cluster protocol.Cluster,
	builder module.Builder,
	communicator hotstuff.Communicator,
	hotstuffModules *consensus.HotstuffModules,
) (module.HotStuff, error) {

	// setup metrics/logging with the new chain ID
	metrics := f.createMetrics(cluster.ChainID())
	builder = blockproducer.NewMetricsWrapper(builder, metrics) // wrapper for measuring time spent building block payload component

	participant, err := consensus.NewParticipant(
		f.log,
		metrics,
		builder,
		communicator,
		hotstuffModules,
		f.opts...,
	)
	return participant, err
}
