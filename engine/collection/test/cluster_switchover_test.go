package test

import (
	"fmt"
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

// ClusterSwitchoverTestCase represents a test case for the cluster switchover
// test suite.
type ClusterSwitchoverTestCase struct {
	clusters   uint // # of clusters each epoch
	collectors uint // # of collectors each epoch
}

// TestClusterSwitchoverSuite runs the cluster switchover test suite.
func TestClusterSwitchoverSuite(t *testing.T) {
	suite.Run(t, new(ClusterSwitchoverSuite))
}

// SetupTest sets up the test based on the given testcase configuration and
// creates mock collection nodes for each configured collector.
func (suite *ClusterSwitchoverSuite) SetupTest(tc ClusterSwitchoverTestCase) {

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

// StartNodes starts all collection nodes in the suite and turns on continuous
// delivery in the stub network.
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

// State returns the protocol state.
func (suite *ClusterSwitchoverSuite) State() protocol.State {
	return suite.nodes[0].State
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

// TestSwitchover runs all cluster switchover test cases.
func (suite *ClusterSwitchoverSuite) TestSwitchover() {
	cases := []ClusterSwitchoverTestCase{
		{
			clusters:   1,
			collectors: 1,
		}, {
			clusters:   1,
			collectors: 2,
		}, {
			clusters:   2,
			collectors: 2,
		}, {
			clusters:   2,
			collectors: 4,
		},
	}

	for _, tc := range cases {
		//suite.Run(fmt.Sprintf("%d clusters, %d collectors", tc.clusters, tc.collectors), func() {
		suite.RunTestCase(tc)
		//})
	}
}

func (suite *ClusterSwitchoverSuite) Test1Collector1Cluster() {
	suite.RunTestCase(ClusterSwitchoverTestCase{
		clusters:   1,
		collectors: 1,
	})
}

func (suite *ClusterSwitchoverSuite) Test2Collectors1() {
	suite.RunTestCase(ClusterSwitchoverTestCase{
		clusters:   1,
		collectors: 2,
	})
}

// TestSwitchover comprises the core test logic for cluster switchover. We build
// an epoch, which triggers the beginning of the epoch 2 cluster consensus, then
// send transactions targeting clusters from both epochs while both are running.
func (suite *ClusterSwitchoverSuite) RunTestCase(tc ClusterSwitchoverTestCase) {
	suite.SetupTest(tc)

	suite.StartNodes()
	defer suite.StopNodes()

	// keep track of guarantees received at the mock consensus node
	// when a guarantee is received, it indicates that the sender has finalized
	// the corresponding cluster block
	expectedGuarantees := int(tc.collectors * 2)
	waitForGuarantees := new(sync.WaitGroup)
	waitForGuarantees.Add(expectedGuarantees)
	suite.sn.On("Submit", mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			// TODO remove logs
			origin := args[0].(flow.Identifier)
			msg := args[1].(*flow.CollectionGuarantee)
			fmt.Println("got message origin: ", origin, " colid: ", msg.ID())
			waitForGuarantees.Done()
		}).
		Times(expectedGuarantees)

	// build the epoch, ending on the first block on the next epoch
	suite.builder.BuildEpoch().CompleteEpoch()

	final, err := suite.State().Final().Head()
	suite.Require().NoError(err)

	epoch1 := suite.State().Final().Epochs().Previous()
	epoch2 := suite.State().Final().Epochs().Current()

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

	// submit transactions targeting epoch 1 clusters
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
		clusterTx := unittest.AlterTransactionForCluster(*tx, epoch1Clustering, cluster, nil)
		epoch2ExpectedTransactions[i] = append(epoch2ExpectedTransactions[i], clusterTx.ID())

		// submit the transaction to any collector in this cluster
		err = suite.Collector(cluster[0].NodeID).IngestionEngine.ProcessLocal(&clusterTx)
		suite.Require().NoError(err)
	}

	unittest.RequireReturnsBefore(suite.T(), waitForGuarantees.Wait, 10*time.Second, "did not receive guarantees at consensus node")

	// check epoch 1 cluster states
	for i, cluster := range epoch1Clusters {
		for _, member := range cluster.Members() {
			node := suite.Collector(member.NodeID)
			state := suite.ClusterState(node, cluster.ChainID())
			expected := epoch1ExpectedTransactions[i]
			unittest.NewClusterStateChecker(state).
				ExpectTxCount(len(expected)).
				ExpectContainsTx(expected...)
		}
	}

	// check epoch 2 cluster states
	for i, cluster := range epoch2Clusters {
		for _, member := range cluster.Members() {
			node := suite.Collector(member.NodeID)
			state := suite.ClusterState(node, cluster.ChainID())
			expected := epoch2ExpectedTransactions[i]
			unittest.NewClusterStateChecker(state).
				ExpectTxCount(len(expected)).
				ExpectContainsTx(expected...)
		}
	}
}
