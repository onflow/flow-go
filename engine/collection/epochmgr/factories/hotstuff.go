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
	"github.com/onflow/flow-go/consensus/hotstuff/timeoutcollector"
	validatorImpl "github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	recovery "github.com/onflow/flow-go/consensus/recovery/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	hotmetrics "github.com/onflow/flow-go/module/metrics/hotstuff"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type HotStuffMetricsFunc func(chainID flow.ChainID) module.HotstuffMetrics

type HotStuffFactory struct {
	baseLogger     zerolog.Logger
	me             module.Local
	db             *badger.DB
	protoState     protocol.State
	engineMetrics  module.EngineMetrics
	mempoolMetrics module.MempoolMetrics
	createMetrics  HotStuffMetricsFunc
	opts           []consensus.Option
}

func NewHotStuffFactory(
	log zerolog.Logger,
	me module.Local,
	db *badger.DB,
	protoState protocol.State,
	engineMetrics module.EngineMetrics,
	mempoolMetrics module.MempoolMetrics,
	createMetrics HotStuffMetricsFunc,
	opts ...consensus.Option,
) (*HotStuffFactory, error) {

	factory := &HotStuffFactory{
		baseLogger:     log,
		me:             me,
		db:             db,
		protoState:     protoState,
		engineMetrics:  engineMetrics,
		mempoolMetrics: mempoolMetrics,
		createMetrics:  createMetrics,
		opts:           opts,
	}
	return factory, nil
}

func (f *HotStuffFactory) CreateModules(
	epoch protocol.Epoch,
	cluster protocol.Cluster,
	clusterState cluster.State,
	headers storage.Headers,
	payloads storage.ClusterPayloads,
	updater module.Finalizer,
) (*consensus.HotstuffModules, module.HotstuffMetrics, error) {
	// setup metrics/logging with the new chain ID
	log := f.createLogger(cluster)
	metrics := f.createMetrics(cluster.ChainID())
	telemetryConsumer := notifications.NewTelemetryConsumer(log)
	slashingConsumer := notifications.NewSlashingViolationsConsumer(log)
	notifier := pubsub.NewDistributor()
	notifier.AddConsumer(notifications.NewLogConsumer(log))
	notifier.AddConsumer(hotmetrics.NewMetricsConsumer(metrics))
	notifier.AddParticipantConsumer(telemetryConsumer)
	notifier.AddProposalViolationConsumer(slashingConsumer)

	var (
		err       error
		committee hotstuff.DynamicCommittee
	)
	committee, err = committees.NewClusterCommittee(f.protoState, payloads, cluster, epoch, f.me.NodeID())
	if err != nil {
		return nil, nil, fmt.Errorf("could not create cluster committee: %w", err)
	}
	committee = committees.NewMetricsWrapper(committee, metrics) // wrapper for measuring time spent determining consensus committee relations

	// create a signing provider
	var signer hotstuff.Signer = verification.NewStakingSigner(f.me)
	signer = verification.NewMetricsWrapper(signer, metrics) // wrapper for measuring time spent with crypto-related operations

	finalizedBlock, err := clusterState.Final().Head()
	if err != nil {
		return nil, nil, fmt.Errorf("could not get cluster finalized block: %w", err)
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
		return nil, nil, err
	}

	voteAggregationDistributor := pubsub.NewVoteAggregationDistributor()
	voteAggregationDistributor.AddVoteCollectorConsumer(telemetryConsumer)
	voteAggregationDistributor.AddVoteAggregationViolationConsumer(slashingConsumer)

	verifier := verification.NewStakingVerifier()
	validator := validatorImpl.NewMetricsWrapper(validatorImpl.New(committee, verifier), metrics)
	voteProcessorFactory := votecollector.NewStakingVoteProcessorFactory(committee, voteAggregationDistributor.OnQcConstructedFromVotes)
	voteAggregator, err := consensus.NewVoteAggregator(
		log,
		metrics,
		f.engineMetrics,
		f.mempoolMetrics,
		// since we don't want to aggregate votes for finalized view,
		// the lowest retained view starts with the next view of the last finalized view.
		finalizedBlock.View+1,
		voteAggregationDistributor,
		voteProcessorFactory,
		notifier.FollowerDistributor,
	)
	if err != nil {
		return nil, nil, err
	}

	timeoutCollectorDistributor := pubsub.NewTimeoutAggregationDistributor()
	timeoutCollectorDistributor.AddTimeoutCollectorConsumer(telemetryConsumer)
	timeoutCollectorDistributor.AddTimeoutAggregationViolationConsumer(slashingConsumer)

	timeoutProcessorFactory := timeoutcollector.NewTimeoutProcessorFactory(log, timeoutCollectorDistributor, committee, validator, msig.CollectorTimeoutTag)
	timeoutAggregator, err := consensus.NewTimeoutAggregator(
		log,
		metrics,
		f.engineMetrics,
		f.mempoolMetrics,
		notifier,
		timeoutProcessorFactory,
		timeoutCollectorDistributor,
		finalizedBlock.View+1,
	)
	if err != nil {
		return nil, nil, err
	}

	return &consensus.HotstuffModules{
		Forks:                       forks,
		Validator:                   validator,
		Notifier:                    notifier,
		Committee:                   committee,
		Signer:                      signer,
		Persist:                     persister.New(f.db, cluster.ChainID()),
		VoteAggregator:              voteAggregator,
		TimeoutAggregator:           timeoutAggregator,
		VoteCollectorDistributor:    voteAggregationDistributor.VoteCollectorDistributor,
		TimeoutCollectorDistributor: timeoutCollectorDistributor.TimeoutCollectorDistributor,
	}, metrics, nil
}

func (f *HotStuffFactory) Create(
	cluster protocol.Cluster,
	clusterState cluster.State,
	metrics module.HotstuffMetrics,
	builder module.Builder,
	headers storage.Headers,
	hotstuffModules *consensus.HotstuffModules,
) (module.HotStuff, error) {

	// setup metrics/logging with the new chain ID
	builder = blockproducer.NewMetricsWrapper(builder, metrics) // wrapper for measuring time spent building block payload component

	finalizedBlock, pendingBlocks, err := recovery.FindLatest(clusterState, headers)
	if err != nil {
		return nil, err
	}

	log := f.createLogger(cluster)
	participant, err := consensus.NewParticipant(
		log,
		metrics,
		f.mempoolMetrics,
		builder,
		finalizedBlock,
		pendingBlocks,
		hotstuffModules,
		f.opts...,
	)
	return participant, err
}

// createLogger creates a logger by wrapping base logger by decorating it will cluster ID
func (f *HotStuffFactory) createLogger(cluster protocol.Cluster) zerolog.Logger {
	return f.baseLogger.With().Str("chain", cluster.ChainID().String()).Logger()
}
