package test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

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

// ClusterSwitchoverTestCase comprises one test case of the cluster switchover.
// Collection nodes are assigned to one cluster each epoch. On epoch
// boundaries they must gracefully terminate cluster consensus for the ending
// epoch and begin cluster consensus the beginning epoch. These two consensus
// committees co-exist for a short period at the beginning of each epoch.
type ClusterSwitchoverTestCase struct {
	t    *testing.T
	conf ClusterSwitchoverTestConf

	identities flow.IdentityList         // identity table
	hub        *stub.Hub                 // mock network hub
	root       protocol.Snapshot         // shared root snapshot
	nodes      []testmock.CollectionNode // collection nodes
	sn         *mockmodule.Engine        // fake consensus node engine for receiving guarantees
	builder    *unittest.EpochBuilder    // utility for building epochs
}

func NewClusterSwitchoverTestCase(t *testing.T, conf ClusterSwitchoverTestConf) *ClusterSwitchoverTestCase {
	return &ClusterSwitchoverTestCase{
		t:    t,
		conf: conf,
	}
}

// TestClusterSwitchover_SingleCluster tests cluster switchover with one cluster.
func TestClusterSwitchover_SingleCluster(t *testing.T) {
	t.Run("1 collector", func(t *testing.T) {
		RunTestCase(NewClusterSwitchoverTestCase(t, ClusterSwitchoverTestConf{
			clusters:   1,
			collectors: 1,
		}))
	})
	t.Run("2 collectors", func(t *testing.T) {
		RunTestCase(NewClusterSwitchoverTestCase(t, ClusterSwitchoverTestConf{
			clusters:   1,
			collectors: 2,
		}))
	})

}

// TestClusterSwitchover_SingleCluster tests cluster switchover with two clusters.
func TestClusterSwitchover_MultiCluster(t *testing.T) {
	RunTestCase(NewClusterSwitchoverTestCase(t, ClusterSwitchoverTestConf{
		clusters:   2,
		collectors: 2,
	}))
}

// ClusterSwitchoverTestConf configures a test case.
type ClusterSwitchoverTestConf struct {
	clusters   uint // # of clusters each epoch
	collectors uint // # of collectors each epoch
}

func (tc *ClusterSwitchoverTestCase) T() *testing.T {
	return tc.t
}

// SetupTest sets up the test based on the given testcase configuration and
// creates mock collection nodes for each configured collector.
func (tc *ClusterSwitchoverTestCase) SetupTest() {

	collectors := unittest.IdentityListFixture(int(tc.conf.collectors), unittest.WithRole(flow.RoleCollection), unittest.WithRandomPublicKeys())
	tc.identities = unittest.CompleteIdentitySet(collectors...)
	tc.hub = stub.NewNetworkHub()

	// create a root snapshot with the given number of initial clusters
	root := unittest.RootSnapshotFixture(tc.identities)
	encodable := root.Encodable()
	setup := encodable.LatestResult.ServiceEvents[0].Event.(*flow.EpochSetup)
	setup.Assignments = unittest.ClusterAssignment(tc.conf.clusters, tc.identities)
	encodable.LatestSeal.ResultID = encodable.LatestResult.ID()
	tc.root = root

	// create a mock node for each collector identity
	for _, collector := range collectors {
		node := testutil.CollectionNode(tc.T(), tc.hub, collector, tc.root)
		tc.nodes = append(tc.nodes, node)
	}

	// create a mock consensus node to receive collection guarantees
	consensus := testutil.GenericNode(
		tc.T(),
		tc.hub,
		tc.identities.Filter(filter.HasRole(flow.RoleConsensus))[0],
		tc.root,
	)
	tc.sn = new(mockmodule.Engine)
	_, err := consensus.Net.Register(engine.ReceiveGuarantees, tc.sn)
	require.NoError(tc.T(), err)

	// create an epoch builder hooked to each collector's protocol state
	states := make([]protocol.MutableState, 0, len(collectors))
	for _, node := range tc.nodes {
		states = append(states, node.State)
	}
	tc.builder = unittest.NewEpochBuilder(tc.T(), states...)
}

// StartNodes starts all collection nodes in the suite and turns on continuous
// delivery in the stub network.
func (tc *ClusterSwitchoverTestCase) StartNodes() {

	// start all node components
	nodes := make([]module.ReadyDoneAware, 0, len(tc.nodes))
	for _, node := range tc.nodes {
		nodes = append(nodes, node)
	}
	unittest.RequireCloseBefore(tc.T(), lifecycle.AllReady(nodes...), time.Second, "could not start nodes")

	// start continuous delivery for all nodes
	for _, node := range tc.nodes {
		node.Net.StartConDev(10*time.Millisecond, false)
	}
}

func (tc *ClusterSwitchoverTestCase) StopNodes() {
	nodes := make([]module.ReadyDoneAware, 0, len(tc.nodes))
	for _, node := range tc.nodes {
		nodes = append(nodes, node)
	}
	unittest.RequireCloseBefore(tc.T(), lifecycle.AllDone(nodes...), time.Second, "could not stop nodes")
}

func (tc *ClusterSwitchoverTestCase) RootBlock() *flow.Header {
	head, err := tc.root.Head()
	require.NoError(tc.T(), err)
	return head
}

func (tc *ClusterSwitchoverTestCase) ServiceAddress() flow.Address {
	return tc.RootBlock().ChainID.Chain().ServiceAddress()
}

// Transaction returns a transaction which is valid for ingestion by a
// collection node in this test suite.
func (tc *ClusterSwitchoverTestCase) Transaction(opts ...func(*flow.TransactionBody)) *flow.TransactionBody {
	tx := flow.NewTransactionBody().
		AddAuthorizer(tc.ServiceAddress()).
		SetPayer(tc.ServiceAddress()).
		SetScript(unittest.NoopTxScript()).
		SetReferenceBlockID(tc.RootBlock().ID())

	for _, apply := range opts {
		apply(tx)
	}

	return tx
}

// ClusterState opens and returns a read-only cluster state for the given node and cluster ID.
func (tc *ClusterSwitchoverTestCase) ClusterState(node testmock.CollectionNode, clusterID flow.ChainID) cluster.State {
	state, err := bcluster.OpenState(node.DB, node.Tracer, node.Headers, node.ClusterPayloads, clusterID)
	require.NoError(tc.T(), err)
	return state
}

// State returns the protocol state.
func (tc *ClusterSwitchoverTestCase) State() protocol.State {
	return tc.nodes[0].State
}

// Collector returns the mock node for the collector with the given ID.
func (tc *ClusterSwitchoverTestCase) Collector(id flow.Identifier) testmock.CollectionNode {
	for _, node := range tc.nodes {
		if node.Me.NodeID() == id {
			return node
		}
	}
	tc.T().FailNow()
	return testmock.CollectionNode{}
}

// Clusters returns the clusters for the current epoch.
func (tc *ClusterSwitchoverTestCase) Clusters(epoch protocol.Epoch) []protocol.Cluster {
	clustering, err := epoch.Clustering()
	require.NoError(tc.T(), err)

	clusters := make([]protocol.Cluster, 0, len(clustering))
	for i := uint(0); i < uint(len(clustering)); i++ {
		cluster, err := epoch.Cluster(i)
		require.NoError(tc.T(), err)
		clusters = append(clusters, cluster)
	}

	return clusters
}

// RunTestCase comprises the core test logic for cluster switchover. We build
// an epoch, which triggers the beginning of the epoch 2 cluster consensus, then
// send transactions targeting clusters from both epochs while both are running.
func RunTestCase(tc *ClusterSwitchoverTestCase) {
	tc.SetupTest()

	tc.StartNodes()
	defer tc.StopNodes()

	// keep track of guarantees received at the mock consensus node
	// when a guarantee is received, it indicates that the sender has finalized
	// the corresponding cluster block
	expectedGuarantees := int(tc.conf.collectors * 2)
	waitForGuarantees := new(sync.WaitGroup)
	waitForGuarantees.Add(expectedGuarantees)
	tc.sn.On("Submit", mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			_, ok := args[0].(flow.Identifier)
			require.True(tc.T(), ok)
			_, ok = args[1].(*flow.CollectionGuarantee)
			require.True(tc.T(), ok)
			waitForGuarantees.Done()
		}).
		Times(expectedGuarantees)

	// build the epoch, ending on the first block on the next epoch
	tc.builder.BuildEpoch().CompleteEpoch()

	final, err := tc.State().Final().Head()
	require.NoError(tc.T(), err)

	epoch1 := tc.State().Final().Epochs().Previous()
	epoch2 := tc.State().Final().Epochs().Current()

	epoch1Clusters := tc.Clusters(epoch1)
	epoch2Clusters := tc.Clusters(epoch2)
	epoch1Clustering, err := epoch1.Clustering()
	require.NoError(tc.T(), err)
	epoch2Clustering, err := epoch2.Clustering()
	require.NoError(tc.T(), err)

	// keep track of which transactions are expected within which cluster for each epoch
	// clusterIndex -> []transactionID
	epoch1ExpectedTransactions := make(map[int][]flow.Identifier)
	epoch2ExpectedTransactions := make(map[int][]flow.Identifier)

	// submit transactions targeting epoch 1 clusters
	for i, clusterMembers := range epoch1Clustering {
		// create a transaction which will be routed to this cluster in epoch 1
		tx := tc.Transaction(func(tx *flow.TransactionBody) {
			tx.SetReferenceBlockID(tc.RootBlock().ID())
		})
		clusterTx := unittest.AlterTransactionForCluster(*tx, epoch1Clustering, clusterMembers, nil)
		epoch1ExpectedTransactions[i] = append(epoch1ExpectedTransactions[i], clusterTx.ID())

		// submit the transaction to any collector in this cluster
		err = tc.Collector(clusterMembers[0].NodeID).IngestionEngine.ProcessLocal(&clusterTx)
		require.NoError(tc.T(), err)
	}

	// submit transactions targeting epoch 2 clusters
	for i, clusterMembers := range epoch2Clustering {
		// create a transaction which will be routed to this cluster in epoch 2
		tx := tc.Transaction(func(tx *flow.TransactionBody) {
			tx.SetReferenceBlockID(final.ID())
		})
		clusterTx := unittest.AlterTransactionForCluster(*tx, epoch1Clustering, clusterMembers, nil)
		epoch2ExpectedTransactions[i] = append(epoch2ExpectedTransactions[i], clusterTx.ID())

		// submit the transaction to any collector in this cluster
		err = tc.Collector(clusterMembers[0].NodeID).IngestionEngine.ProcessLocal(&clusterTx)
		require.NoError(tc.T(), err)
	}

	unittest.RequireReturnsBefore(tc.T(), waitForGuarantees.Wait, 10*time.Second, "did not receive guarantees at consensus node")

	// check epoch 1 cluster states
	for i, clusterInfo := range epoch1Clusters {
		for _, member := range clusterInfo.Members() {
			node := tc.Collector(member.NodeID)
			state := tc.ClusterState(node, clusterInfo.ChainID())
			expected := epoch1ExpectedTransactions[i]
			unittest.NewClusterStateChecker(state).
				ExpectTxCount(len(expected)).
				ExpectContainsTx(expected...).
				Assert(tc.T())
		}
	}

	// check epoch 2 cluster states
	for i, clusterInfo := range epoch2Clusters {
		for _, member := range clusterInfo.Members() {
			node := tc.Collector(member.NodeID)
			state := tc.ClusterState(node, clusterInfo.ChainID())
			expected := epoch2ExpectedTransactions[i]
			unittest.NewClusterStateChecker(state).
				ExpectTxCount(len(expected)).
				ExpectContainsTx(expected...).
				Assert(tc.T())
		}
	}
}
