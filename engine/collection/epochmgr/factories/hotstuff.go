package factories

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/blockproducer"
	"github.com/onflow/flow-go/consensus/hotstuff/committee"
	"github.com/onflow/flow-go/consensus/hotstuff/committee/leader"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/cluster"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	hotmetrics "github.com/onflow/flow-go/module/metrics/hotstuff"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state/cluster"
	clusterkv "github.com/onflow/flow-go/state/cluster/badger"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type HotStuffFactory struct {
	log        zerolog.Logger
	me         module.Local
	db         *badger.DB
	protoState protocol.State
	opts       []consensus.Option
}

func NewHotStuffFactory(
	log zerolog.Logger,
	me module.Local,
	db *badger.DB,
	protoState protocol.State,
	opts ...consensus.Option,
) (*HotStuffFactory, error) {

	factory := &HotStuffFactory{
		log:        log,
		me:         me,
		db:         db,
		protoState: protoState,
		opts:       opts,
	}
	return factory, nil
}

func (f *HotStuffFactory) Create(
	clusterID flow.ChainID,
	cluster flow.IdentityList,
	clusterState cluster.State,
	headers storage.Headers,
	payloads storage.ClusterPayloads,
	seed []byte,
	builder module.Builder,
	updater module.Finalizer,
	communicator hotstuff.Communicator,
	rootHeader *flow.Header,
	rootQC *flow.QuorumCertificate,
) (*hotstuff.EventLoop, error) {

	// setup metrics/logging with the new chain ID
	metrics := metrics.NewHotstuffCollector(clusterID)
	notifier := pubsub.NewDistributor()
	notifier.AddConsumer(notifications.NewLogConsumer(f.log))
	notifier.AddConsumer(hotmetrics.NewMetricsConsumer(metrics))
	notifier.AddConsumer(notifications.NewTelemetryConsumer(f.log, clusterID))
	builder = blockproducer.NewMetricsWrapper(builder, metrics) // wrapper for measuring time spent building block payload component

	selector := filter.And(filter.In(cluster), filter.HasStake(true))
	translator := clusterkv.NewTranslator(payloads, f.protoState)
	selection, err := committee.ComputeLeaderSelectionFromSeed(rootHeader.View, seed, leader.EstimatedSixMonthOfViews, cluster)
	if err != nil {
		return nil, err
	}

	clusterCommittee := committee.New(f.protoState, translator, f.me.NodeID(), selector, cluster.NodeIDs(), selection)
	clusterCommittee = committee.NewMetricsWrapper(clusterCommittee, metrics) // wrapper for measuring time spent determining consensus committee relations

	// create a signing provider
	staking := signature.NewAggregationProvider(encoding.CollectorVoteTag, f.me)
	var signer hotstuff.SignerVerifier = verification.NewSingleSignerVerifier(clusterCommittee, staking, f.me.NodeID())
	signer = verification.NewMetricsWrapper(signer, metrics) // wrapper for measuring time spent with crypto-related operations

	persist := persister.New(f.db, clusterID)

	finalized, pending, err := recovery.FindLatest(clusterState, headers)
	if err != nil {
		return nil, err
	}

	participant, err := consensus.NewParticipant(
		f.log,
		notifier,
		metrics,
		headers,
		clusterCommittee,
		builder,
		updater,
		persist,
		signer,
		communicator,
		rootHeader,
		rootQC,
		finalized,
		pending,
		f.opts...,
	)
	return participant, err
}
