package factories

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus"
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee/leader"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/persister"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	recovery "github.com/dapperlabs/flow-go/consensus/recovery/cluster"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	hotmetrics "github.com/dapperlabs/flow-go/module/metrics/hotstuff"
	"github.com/dapperlabs/flow-go/module/signature"
	"github.com/dapperlabs/flow-go/state/cluster"
	clusterkv "github.com/dapperlabs/flow-go/state/cluster/badger"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
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

	selector := filter.And(filter.In(cluster), filter.HasStake(true))
	translator := clusterkv.NewTranslator(payloads, f.protoState)
	selection, err := committee.ComputeLeaderSelectionFromSeed(rootHeader.View, seed, leader.EstimatedSixMonthOfViews, cluster)
	if err != nil {
		return nil, err
	}

	committee := committee.New(f.protoState, translator, f.me.NodeID(), selector, cluster.NodeIDs(), selection)

	// create a signing provider
	staking := signature.NewAggregationProvider(encoding.CollectorVoteTag, f.me)
	signer := verification.NewSingleSignerVerifier(committee, staking, f.me.NodeID())

	// setup metrics/logging with the new chain ID
	metrics := metrics.NewHotstuffCollector(clusterID)
	notifier := pubsub.NewDistributor()
	notifier.AddConsumer(notifications.NewLogConsumer(f.log))
	notifier.AddConsumer(hotmetrics.NewMetricsConsumer(metrics))

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
		committee,
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
