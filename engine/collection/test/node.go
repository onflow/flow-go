package test

import (
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/engine/collection/epochmgr"
	"github.com/onflow/flow-go/engine/collection/epochmgr/factories"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	epochpool "github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type Node struct {
	t *testing.T

	me    module.Local
	db    *badger.DB
	dbdir string
	state protocol.State
	push  *mocknetwork.Engine

	pools                  *epochpool.TransactionPools
	voter                  module.ClusterRootQCVoter
	clusterStateFactory    *factories.ClusterStateFactory
	builderFactory         *factories.BuilderFactory
	proposalEngineFactory  *factories.ProposalEngineFactory
	syncFactory            *factories.SyncEngineFactory
	hotstuffFactory        *factories.HotStuffFactory
	epochComponentsFactory *factories.EpochComponentsFactory

	epochmgr *epochmgr.Engine
}

func createNode(
	t *testing.T,
	log zerolog.Logger,
	hub *stub.Hub,
	myIdentity *flow.Identity,
	rootSnapshot protocol.Snapshot,
) *Node {

	log = log.With().
		Str("role", myIdentity.Role.String()).
		Str("node_id", myIdentity.NodeID.String()).
		Logger()
	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()

	txLimit := uint(1000)
	create := func() mempool.Transactions { return stdmap.NewTransactions(txLimit) }
	pools := epochpool.NewTransactionPools(create)

	db, dbdir := unittest.TempBadgerDB(t)
	stores := bstorage.InitAll(metrics, db)

	protocolEvents := events.NewDistributor()
	bootstrapState, err := bprotocol.Bootstrap(
		metrics,
		db,
		stores.Headers,
		stores.Seals,
		stores.Results,
		stores.Blocks,
		stores.Setups,
		stores.EpochCommits,
		stores.Statuses,
		rootSnapshot,
	)
	require.NoError(t, err)

	state, err := bprotocol.NewFollowerState(
		bootstrapState,
		stores.Index,
		stores.Payloads,
		tracer,
		protocolEvents,
	)
	require.NoError(t, err)

	me := new(mockmodule.Local)
	me.On("NodeID").Return(myIdentity.NodeID)

	net := stub.NewNetwork(state, me, hub)

	clusterStateFactory, err := factories.NewClusterStateFactory(
		db,
		metrics,
		tracer,
	)
	require.NoError(t, err)

	pusher := new(mocknetwork.Engine)

	builderFactory, err := factories.NewBuilderFactory(
		db,
		stores.Headers,
		tracer,
		metrics,
		pusher,
	)
	require.NoError(t, err)

	proposalFactory, err := factories.NewProposalEngineFactory(
		log,
		net,
		me,
		metrics, metrics, metrics,
		bootstrapState,
		stores.Transactions,
	)
	require.NoError(t, err)

	syncFactory, err := factories.NewSyncEngineFactory(
		log,
		metrics,
		net,
		me,
		synchronization.DefaultConfig(),
	)
	require.NoError(t, err)

	aggregator := new(mockmodule.AggregatingSigner)
	aggregator.On("Sign", mock.Anything).Return(unittest.SignatureFixture(), nil)
	aggregator.On("Aggregate", mock.Anything).Return(unittest.SignatureFixture(), nil)
	aggregator.On("VerifyMany", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	aggregator.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	hotstuffFactory, err := factories.NewHotStuffFactory(
		log,
		me,
		aggregator,
		db,
		bootstrapState,
	)

	// TODO add to HotStuff
	//signer := verification.MockSigner{LocalID: me.NodeID()}
	//_ = signer
	rootQCVoter := new(mockmodule.ClusterRootQCVoter)

	factory := factories.NewEpochComponentsFactory(
		me,
		pools,
		builderFactory,
		clusterStateFactory,
		hotstuffFactory,
		proposalFactory,
		syncFactory,
	)

	heights := gadgets.NewHeights()
	protocolEvents.AddConsumer(heights)

	epochManager, err := epochmgr.New(
		log,
		me,
		state,
		pools,
		rootQCVoter,
		factory,
		heights,
	)
	require.NoError(t, err)

	node := &Node{
		t:                     t,
		me:                    me,
		db:                    db,
		dbdir:                 dbdir,
		state:                 state,
		push:                  pusher,
		pools:                 pools,
		voter:                 rootQCVoter,
		builderFactory:        builderFactory,
		clusterStateFactory:   clusterStateFactory,
		syncFactory:           syncFactory,
		proposalEngineFactory: proposalFactory,
		hotstuffFactory:       hotstuffFactory,
		epochmgr:              epochManager,
	}

	return node
}

func (node *Node) Ready() <-chan struct{} {
	return node.epochmgr.Ready()
}

func (node *Node) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		<-node.epochmgr.Done()
		require.NoError(node.t, node.db.Close())
		require.NoError(node.t, os.RemoveAll(node.dbdir))
		close(done)
	}()
	return done
}
