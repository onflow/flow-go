package test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/testutil"
	testmock "github.com/onflow/flow-go/engine/testutil/mock"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/cluster"
	bcluster "github.com/onflow/flow-go/state/cluster/badger"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
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
	sn         *mocknetwork.Engine       // fake consensus node engine for receiving guarantees
	builder    *unittest.EpochBuilder    // utility for building epochs

	// epoch counter -> cluster index -> transaction IDs
	sentTransactions map[uint64]map[uint]flow.IdentifierList // track submitted transactions
}

// NewClusterSwitchoverTestCase constructs a new cluster switchover test case
// given the configuration, creating all dependencies and mock nodes.
func NewClusterSwitchoverTestCase(t *testing.T, conf ClusterSwitchoverTestConf) *ClusterSwitchoverTestCase {

	tc := &ClusterSwitchoverTestCase{
		t:    t,
		conf: conf,
	}

	nodeInfos := unittest.PrivateNodeInfosFixture(int(conf.collectors), unittest.WithRole(flow.RoleCollection))
	collectors := model.ToIdentityList(nodeInfos)
	tc.identities = unittest.CompleteIdentitySet(collectors...)
	assignment := unittest.ClusterAssignment(tc.conf.clusters, collectors)
	clusters, err := flow.NewClusterList(assignment, collectors)
	require.NoError(t, err)
	rootClusterBlocks := run.GenerateRootClusterBlocks(1, clusters)
	rootClusterQCs := make([]flow.ClusterQCVoteData, len(rootClusterBlocks))
	for i, cluster := range clusters {
		signers := make([]model.NodeInfo, 0)
		for _, identity := range nodeInfos {
			if _, inCluster := cluster.ByNodeID(identity.NodeID); inCluster {
				signers = append(signers, identity)
			}
		}
		qc, err := run.GenerateClusterRootQC(signers, model.ToIdentityList(signers), rootClusterBlocks[i])
		require.NoError(t, err)
		rootClusterQCs[i] = flow.ClusterQCVoteDataFromQC(qc)
	}

	tc.sentTransactions = make(map[uint64]map[uint]flow.IdentifierList)
	tc.hub = stub.NewNetworkHub()

	// create a root snapshot with the given number of initial clusters
	root, result, seal := unittest.BootstrapFixture(tc.identities)
	qc := unittest.QuorumCertificateFixture(unittest.QCWithBlockID(root.ID()))
	setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
	commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)

	setup.Assignments = unittest.ClusterAssignment(tc.conf.clusters, tc.identities)
	commit.ClusterQCs = rootClusterQCs

	seal.ResultID = result.ID()
	tc.root, err = inmem.SnapshotFromBootstrapState(root, result, seal, qc)
	require.NoError(t, err)

	// create a mock node for each collector identity
	for _, collector := range nodeInfos {
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
	tc.sn = new(mocknetwork.Engine)
	_, err = consensus.Net.Register(engine.ReceiveGuarantees, tc.sn)
	require.NoError(tc.T(), err)

	// create an epoch builder hooked to each collector's protocol state
	states := make([]protocol.MutableState, 0, len(collectors))
	for _, node := range tc.nodes {
		states = append(states, node.State)
	}
	// when building new epoch we would like to replace fixture cluster QCs with real ones, for that we need
	// to generate them using node infos
	tc.builder = unittest.NewEpochBuilder(tc.T(), states...).UsingCommitOpts(func(commit *flow.EpochCommit) {
		// build a lookup table for node infos
		nodeInfoLookup := make(map[flow.Identifier]model.NodeInfo)
		for _, nodeInfo := range nodeInfos {
			nodeInfoLookup[nodeInfo.NodeID] = nodeInfo
		}

		// replace cluster QCs, with real data
		for i, clusterQC := range commit.ClusterQCs {
			clusterParticipants := flow.IdentifierList(clusterQC.VoterIDs).Lookup()
			signers := make([]model.NodeInfo, 0, len(clusterParticipants))
			for _, signerID := range clusterQC.VoterIDs {
				signer := nodeInfoLookup[signerID]
				signers = append(signers, signer)
			}
			// generate root cluster block
			rootClusterBlock := cluster.CanonicalRootBlock(commit.Counter, model.ToIdentityList(signers))
			// generate cluster root qc
			qc, err := run.GenerateClusterRootQC(signers, model.ToIdentityList(signers), rootClusterBlock)
			require.NoError(t, err)
			commit.ClusterQCs[i] = flow.ClusterQCVoteDataFromQC(qc)
		}
	})

	return tc
}

// TestClusterSwitchover_Simple is the simplest switchover case with one single-node cluster.
func TestClusterSwitchover_Simple(t *testing.T) {
	RunTestCase(NewClusterSwitchoverTestCase(t, ClusterSwitchoverTestConf{
		clusters:   1,
		collectors: 1,
	}))
}

// TestClusterSwitchover_MultiCollectorCluster tests switchover with a cluster
// containing more than one collector.
func TestClusterSwitchover_MultiCollectorCluster(t *testing.T) {
	RunTestCase(NewClusterSwitchoverTestCase(t, ClusterSwitchoverTestConf{
		clusters:   1,
		collectors: 2,
	}))
}

// TestClusterSwitchover_MultiCluster tests cluster switchover with two clusters.
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

// StartNodes starts all collection nodes in the suite and turns on continuous
// delivery in the stub network.
func (tc *ClusterSwitchoverTestCase) StartNodes() {

	// start all node components
	nodes := make([]module.ReadyDoneAware, 0, len(tc.nodes))
	for _, node := range tc.nodes {
		nodes = append(nodes, node)
	}
	unittest.RequireCloseBefore(tc.T(), util.AllReady(nodes...), time.Second, "could not start nodes")

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
	unittest.RequireCloseBefore(tc.T(), util.AllDone(nodes...), time.Second, "could not stop nodes")
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

// ExpectTransaction asserts that the test case expects the given transaction
// to be included in the given cluster state for the given epoch.
func (tc *ClusterSwitchoverTestCase) ExpectTransaction(epochCounter uint64, clusterIndex uint, txID flow.Identifier) {
	if _, ok := tc.sentTransactions[epochCounter]; !ok {
		tc.sentTransactions[epochCounter] = make(map[uint]flow.IdentifierList)
	}
	tc.T().Logf("expecting transaction %x in epoch %d for cluster %d", txID, epochCounter, clusterIndex)
	expected := tc.sentTransactions[epochCounter][clusterIndex]
	expected = append(expected, txID)
	tc.sentTransactions[epochCounter][clusterIndex] = expected
}

// ClusterState opens and returns a read-only cluster state for the given node and cluster ID.
func (tc *ClusterSwitchoverTestCase) ClusterState(node testmock.CollectionNode, clusterID flow.ChainID) cluster.State {
	state, err := bcluster.OpenState(node.PublicDB, node.Tracer, node.Headers, node.ClusterPayloads, clusterID)
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

// BlockInEpoch returns the highest block that exists within the bounds of the
// epoch with the given epoch counter.
func (tc *ClusterSwitchoverTestCase) BlockInEpoch(epochCounter uint64) *flow.Header {
	root := tc.RootBlock()

	for height := root.Height; ; height++ {
		curr := tc.State().AtHeight(height)
		next := tc.State().AtHeight(height + 1)
		curCounter, err := curr.Epochs().Current().Counter()
		require.NoError(tc.T(), err)
		nextCounter, err := next.Epochs().Current().Counter()
		// if we reach a point where the next block doesn't exist, but the
		// current block has the correct counter, return the current block
		if err != nil && curCounter == epochCounter {
			head, err := curr.Head()
			require.NoError(tc.T(), err)
			return head
		}

		// otherwise, wait until we reach the block where the next block is in
		// the next epoch - this is the highest block in the requested epoch
		if curCounter == epochCounter && nextCounter == epochCounter+1 {
			head, err := curr.Head()
			require.NoError(tc.T(), err)
			return head
		}
	}
}

// SubmitTransactionToCluster submits a transaction to the given cluster in
// the given epoch and marks the transaction as expected for inclusion in
// the corresponding cluster state.
func (tc *ClusterSwitchoverTestCase) SubmitTransactionToCluster(
	epochCounter uint64, // the epoch we are submitting the transacting w.r.t.
	clustering flow.ClusterList, // the clustering for the epoch
	clusterIndex uint, // the index of the cluster we are targetting
) {

	clusterMembers := clustering[int(clusterIndex)]
	// get any block within the target epoch as the transaction's reference block
	refBlock := tc.BlockInEpoch(epochCounter)
	tx := tc.Transaction(func(tx *flow.TransactionBody) {
		tx.SetReferenceBlockID(refBlock.ID())
	})
	clusterTx := unittest.AlterTransactionForCluster(*tx, clustering, clusterMembers, nil)
	tc.ExpectTransaction(epochCounter, clusterIndex, clusterTx.ID())

	// submit the transaction to any collector in this cluster
	err := tc.Collector(clusterMembers[0].NodeID).IngestionEngine.ProcessTransaction(&clusterTx)
	require.NoError(tc.T(), err)
}

// CheckClusterState checks the cluster state of the given node (within the given
// cluster) and asserts that only transaction specified by ExpectTransaction are
// included.
func (tc *ClusterSwitchoverTestCase) CheckClusterState(
	identity *flow.Identity,
	clusterInfo protocol.Cluster,
) {
	node := tc.Collector(identity.NodeID)
	state := tc.ClusterState(node, clusterInfo.ChainID())
	expected := tc.sentTransactions[clusterInfo.EpochCounter()][clusterInfo.Index()]
	unittest.NewClusterStateChecker(state).
		ExpectTxCount(len(expected)).
		ExpectContainsTx(expected...).
		Assert(tc.T())
}

// Timeout returns the timeout for async tasks for this test case.
func (tc *ClusterSwitchoverTestCase) Timeout() time.Duration {
	// 60s + 10s for each collector
	// locally the whole suite takes
	// * ~8s when run alone
	// * ~15-20s when run in parallel with other packages (default)
	return 60*time.Second + 10*time.Second*time.Duration(tc.conf.collectors)
}

// RunTestCase comprises the core test logic for cluster switchover. We build
// an epoch, which triggers the beginning of the epoch 2 cluster consensus, then
// send transactions targeting clusters from both epochs while both are running.
func RunTestCase(tc *ClusterSwitchoverTestCase) {

	tc.StartNodes()
	defer tc.StopNodes()

	// keep track of guarantees received at the mock consensus node
	// when a guarantee is received, it indicates that the sender has finalized
	// the corresponding cluster block
	expectedGuaranteesPerEpoch := int(tc.conf.collectors)
	waitForGuarantees := new(sync.WaitGroup)
	waitForGuarantees.Add(expectedGuaranteesPerEpoch)
	tc.sn.On("Process", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			id, ok := args[1].(flow.Identifier)
			require.True(tc.T(), ok)
			_, ok = args[2].(*flow.CollectionGuarantee)
			tc.T().Log("got guarantee from", id.String())
			require.True(tc.T(), ok)
			waitForGuarantees.Done()
		}).
		Times(expectedGuaranteesPerEpoch * 2)

	// build the epoch, ending on the first block on the next epoch
	tc.builder.BuildEpoch().CompleteEpoch()
	// build halfway through the grace period for the epoch 1 cluster
	tc.builder.BuildBlocks(flow.DefaultTransactionExpiry / 2)

	epoch1 := tc.State().Final().Epochs().Previous()
	epoch2 := tc.State().Final().Epochs().Current()

	epoch1Clusters := tc.Clusters(epoch1)
	epoch2Clusters := tc.Clusters(epoch2)
	epoch1Clustering, err := epoch1.Clustering()
	require.NoError(tc.T(), err)
	epoch2Clustering, err := epoch2.Clustering()
	require.NoError(tc.T(), err)

	// submit transactions targeting epoch 1 clusters
	for clusterIndex := range epoch1Clustering {
		tc.SubmitTransactionToCluster(1, epoch1Clustering, uint(clusterIndex))
	}

	// wait for epoch 1 transactions to be guaranteed
	unittest.RequireReturnsBefore(tc.T(), waitForGuarantees.Wait, tc.Timeout(), "did not receive guarantees at consensus node")

	// submit transactions targeting epoch 2 clusters
	for clusterIndex := range epoch2Clustering {
		tc.SubmitTransactionToCluster(2, epoch2Clustering, uint(clusterIndex))
	}

	waitForGuarantees.Add(expectedGuaranteesPerEpoch)

	// build enough blocks to terminate the epoch 1 cluster consensus
	// NOTE: this is here solely to improve test reliability, as it means that
	// while we are waiting for a guarantee there is only one cluster consensus
	// instance running (per node) rather than two.
	tc.builder.BuildBlocks(flow.DefaultTransactionExpiry/2 + 1)

	// wait for epoch 2 transactions to be guaranteed
	unittest.RequireReturnsBefore(tc.T(), waitForGuarantees.Wait, tc.Timeout(), "did not receive guarantees at consensus node")

	// check epoch 1 cluster states
	for _, clusterInfo := range epoch1Clusters {
		for _, member := range clusterInfo.Members() {
			tc.CheckClusterState(member, clusterInfo)
		}
	}

	// check epoch 2 cluster states
	for _, clusterInfo := range epoch2Clusters {
		for _, member := range clusterInfo.Members() {
			tc.CheckClusterState(member, clusterInfo)
		}
	}
}
