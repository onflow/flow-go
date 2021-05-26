package test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/testutil"
	testmock "github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/lifecycle"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/cluster"
	bcluster "github.com/onflow/flow-go/state/cluster/badger"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

// ClusterSwitchoverSuite is the test suite for testing cluster switchover at
// epoch boundaries. Collection nodes are assigned to one cluster each epoch.
// On epoch boundaries they must gracefully terminate cluster consensus for
// the ending epoch and begin cluster consensus the beginning epoch. These two
// consensus committees co-exist for a short period at the beginning of each
// epoch.
type ClusterSwitchoverSuite struct {
	suite.Suite

	identities flow.IdentityList         // identity table
	hub        *stub.Hub                 // mock network hub
	root       protocol.Snapshot         // shared root snapshot
	nodes      []testmock.CollectionNode // collection nodes
	sn         *mockmodule.Engine        // fake consensus node engine for receiving guarantees
	builder    *unittest.EpochBuilder    // utility for building epochs
}

func TestClusterSwitchoverSuite(t *testing.T) {
	suite.Run(t, new(ClusterSwitchoverSuite))
}

// SetupTest sets up the test with the given identity table, creating mock
// nodes for all collection nodes.
func (suite *ClusterSwitchoverSuite) SetupTest(tc testcase) {
	collectors := unittest.IdentityListFixture(int(tc.collectors), unittest.WithRole(flow.RoleCollection), unittest.WithRandomPublicKeys())
	suite.identities = unittest.CompleteIdentitySet(collectors...)
	suite.hub = stub.NewNetworkHub()

	// create a root snapshot with the given number of initial clusters
	root := unittest.RootSnapshotFixture(suite.identities)
	encodable := root.Encodable()
	setup := encodable.LatestResult.ServiceEvents[0].Event.(*flow.EpochSetup)
	setup.Assignments = unittest.ClusterAssignment(tc.clusters, suite.identities)
	encodable.LatestSeal.ResultID = encodable.LatestResult.ID()
	suite.root = root

	// create a mock node for each collector identity
	for _, collector := range collectors {
		node := testutil.CollectionNode(suite.T(), suite.hub, collector, suite.root)
		suite.nodes = append(suite.nodes, node)
	}

	// create a mock consensus node to receive collection guarantees
	consensus := testutil.GenericNode(
		suite.T(),
		suite.hub,
		suite.identities.Filter(filter.HasRole(flow.RoleConsensus))[0],
		suite.root,
	)
	suite.sn = new(mockmodule.Engine)
	_, err := consensus.Net.Register(engine.ReceiveGuarantees, suite.sn)
	require.NoError(suite.T(), err)

	// create an epoch builder hooked to each collector's protocol state
	states := make([]protocol.MutableState, 0, len(collectors))
	for _, node := range suite.nodes {
		states = append(states, node.State)
	}
	suite.builder = unittest.NewEpochBuilder(suite.T(), states...)
}

func (suite *ClusterSwitchoverSuite) StartNodes() {

	// start all node components
	nodes := make([]module.ReadyDoneAware, 0, len(suite.nodes))
	for _, node := range suite.nodes {
		nodes = append(nodes, node)
	}
	unittest.RequireCloseBefore(suite.T(), lifecycle.AllReady(nodes...), time.Second, "could not start nodes")

	// start continuous delivery for all nodes
	for _, node := range suite.nodes {
		node.Net.StartConDev(10*time.Millisecond, false)
	}
}

func (suite *ClusterSwitchoverSuite) StopNodes() {
	nodes := make([]module.ReadyDoneAware, 0, len(suite.nodes))
	for _, node := range suite.nodes {
		nodes = append(nodes, node)
	}
	unittest.RequireCloseBefore(suite.T(), lifecycle.AllDone(nodes...), time.Second, "could not stop nodes")
}

func (suite *ClusterSwitchoverSuite) RootBlock() *flow.Header {
	head, err := suite.root.Head()
	require.NoError(suite.T(), err)
	return head
}

func (suite *ClusterSwitchoverSuite) ServiceAddress() flow.Address {
	return suite.RootBlock().ChainID.Chain().ServiceAddress()
}

// Transaction returns a transaction which is valid for ingestion by a
// collection node in this test suite.
func (suite *ClusterSwitchoverSuite) Transaction(opts ...func(*flow.TransactionBody)) *flow.TransactionBody {
	tx := flow.NewTransactionBody().
		AddAuthorizer(suite.ServiceAddress()).
		SetPayer(suite.ServiceAddress()).
		SetScript(unittest.NoopTxScript()).
		SetReferenceBlockID(suite.RootBlock().ID())

	for _, apply := range opts {
		apply(tx)
	}

	return tx
}

// ClusterState opens and returns a read-only cluster state for the given node and cluster ID.
func (suite *ClusterSwitchoverSuite) ClusterState(node testmock.CollectionNode, clusterID flow.ChainID) cluster.State {
	state, err := bcluster.OpenState(node.DB, node.Tracer, node.Headers, node.ClusterPayloads, clusterID)
	suite.Require().NoError(err)
	return state
}

// TestSingleNode tests cluster switchover with a single node.
func (suite *ClusterSwitchoverSuite) TestSingleNode() {

	tc := testcase{
		clusters:   1,
		collectors: 1,
	}
	suite.SetupTest(tc)

	// start the nodes
	suite.StartNodes()
	defer suite.StopNodes()

	collector := suite.nodes[0]

	// build out the first epoch and begin the second epoch - at this point
	// the collection node should be running consensus for both epochs 1 & 2
	builder := unittest.NewEpochBuilder(suite.T(), collector.State)
	builder.BuildEpoch().CompleteEpoch()

	final, err := collector.State.Final().Head()
	require.NoError(suite.T(), err)

	// create one transaction for each epoch
	epoch1Tx := suite.Transaction(func(tx *flow.TransactionBody) {
		// reference a block in epoch 1, so it is routed to the epoch 1 cluster
		tx.SetReferenceBlockID(suite.RootBlock().ID())
	})
	epoch2Tx := suite.Transaction(func(tx *flow.TransactionBody) {
		// reference a block in epoch 2, so it is routed to the epoch 2 cluster
		tx.SetReferenceBlockID(final.ID())
	})

	// expect 2 collections to be received
	waitForGuarantees := new(sync.WaitGroup)
	waitForGuarantees.Add(2)
	suite.sn.On("Submit", mock.Anything, mock.Anything).
		Return(nil).
		Run(func(_ mock.Arguments) {
			waitForGuarantees.Done()
		}).
		Twice()

	// submit both transactions to the node's ingestion engine
	err = collector.IngestionEngine.ProcessLocal(epoch1Tx)
	require.NoError(suite.T(), err)
	err = collector.IngestionEngine.ProcessLocal(epoch2Tx)
	require.NoError(suite.T(), err)

	// wait for 2 distinct collection guarantees to be received by the consensus node
	unittest.RequireReturnsBefore(suite.T(), waitForGuarantees.Wait, time.Second, "did not receive 2 collections")

	// open cluster states for both epoch 1 and epoch 2 clusters
	epoch1Cluster, err := collector.State.Final().Epochs().Previous().Cluster(0)
	suite.Require().NoError(err)
	epoch2Cluster, err := collector.State.Final().Epochs().Current().Cluster(0)
	suite.Require().NoError(err)
	epoch1ClusterState := suite.ClusterState(collector, epoch1Cluster.ChainID())
	epoch2ClusterState := suite.ClusterState(collector, epoch2Cluster.ChainID())

	// the transaction referencing a block in epoch 1 should be in the epoch 1 cluster state
	unittest.NewClusterStateChecker(epoch1ClusterState).
		ExpectTxCount(1).
		ExpectContainsTx(epoch1Tx.ID()).
		ExpectOmitsTx(epoch2Tx.ID()).
		Assert(suite.T())

	// the transaction referencing a block in epoch 2 should be in the epoch 2 cluster state
	unittest.NewClusterStateChecker(epoch2ClusterState).
		ExpectTxCount(1).
		ExpectContainsTx(epoch2Tx.ID()).
		ExpectOmitsTx(epoch1Tx.ID()).
		Assert(suite.T())
}

type testcase struct {
	clusters   uint
	collectors uint
}

func (suite *ClusterSwitchoverSuite) AnyCollector() testmock.CollectionNode {
	return suite.nodes[0]
}

// Collector returns the mock node for the collector with the given ID.
func (suite *ClusterSwitchoverSuite) Collector(id flow.Identifier) testmock.CollectionNode {
	for _, node := range suite.nodes {
		if node.Me.NodeID() == id {
			return node
		}
	}
	suite.T().FailNow()
	return testmock.CollectionNode{}
}

// Clusters returns the clusters for the current epoch.
func (suite *ClusterSwitchoverSuite) Clusters(epoch protocol.Epoch) []protocol.Cluster {
	clustering, err := epoch.Clustering()
	suite.Require().NoError(err)

	clusters := make([]protocol.Cluster, 0, len(clustering))
	for i := uint(0); i < uint(len(clustering)); i++ {
		cluster, err := epoch.Cluster(i)
		suite.Require().NoError(err)
		clusters = append(clusters, cluster)
	}

	return clusters
}

func (suite *ClusterSwitchoverSuite) TestSwitchoverInstance() {
	tc := testcase{
		clusters:   1,
		collectors: 2,
	}
	suite.TestSwitchover(tc)
}

func (suite *ClusterSwitchoverSuite) TestSwitchover(tc testcase) {
	suite.SetupTest(tc)

	suite.StartNodes()
	defer suite.StopNodes()

	// TODO check this value
	expectedGuarantees := int(tc.collectors * 2)
	waitForGuarantees := new(sync.WaitGroup)
	waitForGuarantees.Add(expectedGuarantees)
	suite.sn.On("Submit", mock.Anything, mock.Anything).
		Return(nil).
		Run(func(_ mock.Arguments) {
			waitForGuarantees.Done()
		}).
		Times(expectedGuarantees)

	// EPOCH 1

	// build the epoch, ending on the first block on the next epoch
	suite.builder.BuildEpoch().CompleteEpoch()

	final, err := suite.AnyCollector().State.Final().Head()
	suite.Require().NoError(err)

	epoch1 := suite.AnyCollector().State.Final().Epochs().Previous()
	epoch2 := suite.AnyCollector().State.Final().Epochs().Current()

	epoch1Clusters := suite.Clusters(epoch1)
	epoch2Clusters := suite.Clusters(epoch2)
	epoch1Clustering, err := epoch1.Clustering()
	suite.Require().NoError(err)
	epoch2Clustering, err := epoch2.Clustering()
	suite.Require().NoError(err)

	// keep track of which transactions are expected within which cluster for each epoch
	// clusterIndex -> []transactionID
	epoch1ExpectedTransactions := make(map[int][]flow.Identifier)
	epoch2ExpectedTransactions := make(map[int][]flow.Identifier)

	// submit transactions targeting epoch 2 clusters
	for i, cluster := range epoch1Clustering {
		// create a transaction which will be routed to this cluster in epoch 1
		tx := suite.Transaction(func(tx *flow.TransactionBody) {
			tx.SetReferenceBlockID(suite.RootBlock().ID())
		})
		clusterTx := unittest.AlterTransactionForCluster(*tx, epoch1Clustering, cluster, nil)
		epoch1ExpectedTransactions[i] = append(epoch1ExpectedTransactions[i], clusterTx.ID())

		// submit the transaction to any collector in this cluster
		err = suite.Collector(cluster[0].NodeID).IngestionEngine.ProcessLocal(&clusterTx)
		suite.Require().NoError(err)
	}

	// submit transactions targeting epoch 2 clusters
	for i, cluster := range epoch2Clustering {
		// create a transaction which will be routed to this cluster in epoch 2
		tx := suite.Transaction(func(tx *flow.TransactionBody) {
			tx.SetReferenceBlockID(final.ID())
		})
		// TODO - note this down for cluster state checker later
		clusterTx := unittest.AlterTransactionForCluster(*tx, epoch1Clustering, cluster, nil)
		epoch2ExpectedTransactions[i] = append(epoch2ExpectedTransactions[i], clusterTx.ID())

		// submit the transaction to any collector in this cluster
		err = suite.Collector(cluster[0].NodeID).IngestionEngine.ProcessLocal(&clusterTx)
		suite.Require().NoError(err)
	}

	unittest.RequireReturnsBefore(suite.T(), waitForGuarantees.Wait, time.Second, "did not receive guarantees at consensus node")

	// check epoch 1 cluster states
	for i, cluster := range epoch1Clusters {
		for _, member := range cluster.Members() {
			node := suite.Collector(member.NodeID)
			state := suite.ClusterState(node, cluster.ChainID())
			unittest.NewClusterStateChecker(state).
				ExpectTxCount(1).
				ExpectContainsTx(epoch1ExpectedTransactions[i]...)
		}
	}

	// check epoch 2 cluster states
	for i, cluster := range epoch2Clusters {
		for _, member := range cluster.Members() {
			node := suite.Collector(member.NodeID)
			state := suite.ClusterState(node, cluster.ChainID())
			unittest.NewClusterStateChecker(state).
				ExpectTxCount(1).
				ExpectContainsTx(epoch2ExpectedTransactions[i]...)
		}
	}
}
