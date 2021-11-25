package factories

import (
	"fmt"
	"github.com/gammazero/workerpool"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"

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

func (f *HotStuffFactory) Create(
	epoch protocol.Epoch,
	cluster protocol.Cluster,
	clusterState cluster.State,
	headers storage.Headers,
	payloads storage.ClusterPayloads,
	builder module.Builder,
	updater module.Finalizer,
	communicator hotstuff.Communicator,
) (module.HotStuff, error) {

	// setup metrics/logging with the new chain ID
	metrics := f.createMetrics(cluster.ChainID())
	notifier := pubsub.NewDistributor()
	notifier.AddConsumer(notifications.NewLogConsumer(f.log))
	notifier.AddConsumer(hotmetrics.NewMetricsConsumer(metrics))
	notifier.AddConsumer(notifications.NewTelemetryConsumer(f.log, cluster.ChainID()))
	builder = blockproducer.NewMetricsWrapper(builder, metrics) // wrapper for measuring time spent building block payload component

	hotstuffModules := &consensus.HotstuffModules{
		Forks:                nil,
		Validator:            nil,
		Notifier:             notifier,
		Committee:            nil,
		Signer:               nil,
		Persist:              persister.New(f.db, cluster.ChainID()),
		Aggregator:           nil,
		QCCreatedDistributor: pubsub.NewQCCreatedDistributor(),
	}

	var err error
	hotstuffModules.Committee, err = committees.NewClusterCommittee(f.protoState, payloads, cluster, epoch, f.me.NodeID())
	if err != nil {
		return nil, fmt.Errorf("could not create cluster committee: %w", err)
	}
	hotstuffModules.Committee = committees.NewMetricsWrapper(hotstuffModules.Committee, metrics) // wrapper for measuring time spent determining consensus committee relations

	// create a signing provider
	hotstuffModules.Signer = verification.NewSingleSignerVerifier(hotstuffModules.Committee, f.aggregator, f.me.NodeID())
	hotstuffModules.Signer = verification.NewMetricsWrapper(hotstuffModules.Signer, metrics) // wrapper for measuring time spent with crypto-related operations

	finalized, pending, err := recovery.FindLatest(clusterState, headers)
	if err != nil {
		return nil, err
	}

	hotstuffModules, err = consensus.InitForks(
		finalized,
		headers,
		updater,
		hotstuffModules,
		cluster.RootBlock().Header,
		cluster.RootQC(),
	)
	if err != nil {
		return nil, err
	}
	hotstuffModules = consensus.InitValidator(metrics, hotstuffModules)

	voteProcessorFactory := votecollector.NewStakingVoteProcessorFactory(hotstuffModules.Committee,
		hotstuffModules.QCCreatedDistributor.OnQcConstructedFromVotes)

	workerPool := workerpool.New(4)
	hotstuffModules.Aggregator, err = consensus.NewVoteAggregator(f.log, finalized, pending, hotstuffModules, workerPool,
		voteProcessorFactory)
	if err != nil {
		return nil, err
	}

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
