package collection

import (
	"context"
	"testing"
	"time"

	sdk "github.com/onflow/flow-go-sdk"
	sdkclient "github.com/onflow/flow-go-sdk/client"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	ghostclient "github.com/dapperlabs/flow-go/engine/ghost/client"
	"github.com/dapperlabs/flow-go/integration/convert"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/integration/tests/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	cluster "github.com/dapperlabs/flow-go/state/cluster/badger"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const (
	// the default timeout for individual actions (eg. send a transaction)
	defaultTimeout = 10 * time.Second
)

// CollectorSuite represents a test suite for collector nodes.
type CollectorSuite struct {
	suite.Suite

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net       *testnet.FlowNetwork
	nClusters uint

	// account info
	acct struct {
		key    *sdk.AccountKey
		addr   sdk.Address
		signer sdkcrypto.Signer
	}

	// ghost node
	ghostID flow.Identifier
	reader  *ghostclient.FlowMessageStreamReader
}

func TestCollectorSuite(t *testing.T) {
	suite.Run(t, new(CollectorSuite))
}

// SetupTest generates a test network with the given number of collector nodes
// and clusters and starts the network.
//
// NOTE: This must be called explicitly by each test, since nodes/clusters vary
//       between test cases.
func (suite *CollectorSuite) SetupTest(name string, nNodes, nClusters uint) {

	// default set of non-collector nodes
	var (
		conNode = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.ErrorLevel), testnet.AsGhost())
		exeNode = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.ErrorLevel), testnet.AsGhost())
		verNode = testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.ErrorLevel), testnet.AsGhost())
	)
	colNodes := testnet.NewNodeConfigSet(nNodes, flow.RoleCollection)

	suite.nClusters = nClusters

	// set one of the non-collector nodes to be the ghost
	suite.ghostID = conNode.Identifier

	// instantiate the network
	nodes := append(colNodes, conNode, exeNode, verNode)
	conf := testnet.NewNetworkConfig(name, nodes, testnet.WithClusters(nClusters))
	suite.net = testnet.PrepareFlowNetwork(suite.T(), conf)

	// start the network
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.net.Start(suite.ctx)

	// create an account to use for sending transactions
	suite.acct.addr, suite.acct.key, suite.acct.signer = common.GetAccount()

	// subscribe to the ghost
	for attempts := 0; ; attempts++ {
		reader, err := suite.Ghost().Subscribe(suite.ctx)
		if err == nil {
			suite.reader = reader
			break
		}
		if attempts >= 10 {
			require.NoError(suite.T(), err, "could not subscribe to ghost (%d attempts)", attempts)
		}
	}
}

func (suite *CollectorSuite) TearDownTest() {
	suite.net.Remove()
	suite.cancel()
}

// Ghost returns a client for the ghost node.
func (suite *CollectorSuite) Ghost() *ghostclient.GhostClient {
	ghost := suite.net.ContainerByID(suite.ghostID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(suite.T(), err, "could not get ghost client")
	return client
}

func (suite *CollectorSuite) Clusters() *flow.ClusterList {
	identities := suite.net.Identities()
	clusters := protocol.Clusters(suite.nClusters, identities)
	return clusters
}

// NextTransaction returns a valid no-op transaction and updates the
// account key sequence number.
func (suite *CollectorSuite) NextTransaction(opts ...func(*sdk.Transaction)) *sdk.Transaction {
	acct := suite.acct

	tx := sdk.NewTransaction().
		SetScript(unittest.NoopTxScript()).
		SetReferenceBlockID(convert.ToSDKID(suite.net.Genesis().ID())).
		SetProposalKey(acct.addr, acct.key.ID, acct.key.SequenceNumber).
		SetPayer(acct.addr).
		AddAuthorizer(acct.addr)

	for _, apply := range opts {
		apply(tx)
	}

	err := tx.SignEnvelope(acct.addr, acct.key.ID, acct.signer)
	require.Nil(suite.T(), err)

	suite.acct.key.SequenceNumber++

	return tx
}

func (suite *CollectorSuite) TxForCluster(target flow.IdentityList) *sdk.Transaction {

	acct := suite.acct
	clusters := suite.Clusters()
	tx := suite.NextTransaction()

	// hash-grind the script until the transaction will be routed to target cluster
	for {
		tx.SetScript(append(tx.Script, '/', '/'))
		err := tx.SignEnvelope(acct.addr, acct.key.ID, acct.signer)
		require.Nil(suite.T(), err)
		routed := clusters.ByTxID(convert.IDFromSDK(tx.ID()))
		if routed.Fingerprint() == target.Fingerprint() {
			break
		}
	}

	return tx
}

func (suite *CollectorSuite) AwaitTransactionIncluded(txID flow.Identifier) {

	// the height at which the collection is included
	var height uint64

	waitFor := time.Second * 30
	deadline := time.Now().Add(waitFor)
	for time.Now().Before(deadline) {

		_, msg, err := suite.reader.Next()
		require.Nil(suite.T(), err, "could not read next message")
		suite.T().Logf("ghost recv: %T", msg)

		switch val := msg.(type) {
		case *messages.ClusterBlockProposal:
			header := val.Header
			collection := val.Payload.Collection
			suite.T().Logf("got proposal height=%d col_id=%x size=%d", header.Height, collection.ID(), collection.Len())

			// note the height when transaction is includedj
			if collection.Light().Has(txID) {
				height = header.Height
			}

			// use building two blocks as an indication of inclusion
			// TODO replace this with finalization by listening to guarantees
			if height > 0 && header.Height-height >= 2 {
				return
			}

		case *flow.CollectionGuarantee:
			// TODO use this as indication of finalization w/ HotStuff
		}
	}

	suite.T().Logf("timed out waiting for inclusion (timeout=%s, txid=%s)", waitFor.String(), txID)
	suite.T().FailNow()
}

// Collector returns the collector node with the given index in the
// given cluster.
func (suite *CollectorSuite) Collector(clusterIdx, nodeIdx uint) *testnet.Container {

	clusters := suite.Clusters()
	require.True(suite.T(), clusterIdx < uint(clusters.Size()), "invalid cluster index")

	cluster := clusters.ByIndex(clusterIdx)
	node, ok := cluster.ByIndex(nodeIdx)
	require.True(suite.T(), ok, "invalid node index")

	return suite.net.ContainerByID(node.ID())
}

// ClusterStateFor returns a cluster state instance for the collector node
// with the given ID.
func (suite *CollectorSuite) ClusterStateFor(id flow.Identifier) *cluster.State {

	myCluster, ok := suite.Clusters().ByNodeID(id)
	require.True(suite.T(), ok, "could not get node %s in clusters", id)

	chainID := protocol.ChainIDForCluster(myCluster)
	node := suite.net.ContainerByID(id)

	db, err := node.DB()
	require.Nil(suite.T(), err, "could not get node db")

	state, err := cluster.NewState(db, chainID)
	require.Nil(suite.T(), err, "could not get cluster state")

	return state
}

// Test sending various invalid transactions to a single-cluster configuration.
// The transactions should be rejected by the collection node and not included
// in any collection.
func (suite *CollectorSuite) TestTransactionIngress_InvalidTransaction() {
	t := suite.T()

	suite.SetupTest("col_txingress_invalid", 3, 1)

	// pick a collector to test against
	col1 := suite.Collector(0, 0)

	client, err := sdkclient.New(col1.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure())
	require.Nil(t, err)

	t.Run("missing reference block id", func(t *testing.T) {
		malformed := suite.NextTransaction(func(tx *sdk.Transaction) {
			tx.SetReferenceBlockID(sdk.ZeroID)
		})

		expected := ingest.IncompleteTransactionError{
			Missing: []string{flow.TransactionFieldRefBlockID.String()},
		}

		ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
		defer cancel()
		err := client.SendTransaction(ctx, *malformed)
		unittest.AssertErrSubstringMatch(t, expected, err)
	})

	t.Run("missing script", func(t *testing.T) {
		malformed := suite.NextTransaction(func(tx *sdk.Transaction) {
			tx.SetScript(nil)
		})

		expected := ingest.IncompleteTransactionError{
			Missing: []string{flow.TransactionFieldScript.String()},
		}

		ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
		defer cancel()
		err := client.SendTransaction(ctx, *malformed)
		unittest.AssertErrSubstringMatch(t, expected, err)
	})
	t.Run("expired transaction", func(t *testing.T) {
		// TODO blocked by https://github.com/dapperlabs/flow-go/issues/3005
		t.Skip()
	})
	t.Run("non-existent reference block ID", func(t *testing.T) {
		// TODO blocked by https://github.com/dapperlabs/flow-go/issues/3005
		t.Skip()
	})
	t.Run("unparseable script", func(t *testing.T) {
		// TODO script parsing not implemented
		t.Skip()
	})
	t.Run("invalid signature", func(t *testing.T) {
		// TODO signature validation not implemented
		t.Skip()
	})
	t.Run("invalid sequence number", func(t *testing.T) {
		// TODO nonce validation not implemented
		t.Skip()
	})
	t.Run("insufficient payer balance", func(t *testing.T) {
		// TODO balance checking not implemented
		t.Skip()
	})
}

// Test sending a single valid transaction to a single cluster.
// The transaction should be included in a collection.
func (suite *CollectorSuite) TestTxIngress_SingleCluster() {
	t := suite.T()

	suite.SetupTest("col_txingress_singlecluster", 3, 1)

	// pick a collector to test against
	col1 := suite.Collector(0, 0)

	client, err := sdkclient.New(col1.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure())
	require.Nil(t, err)

	tx := suite.NextTransaction()
	require.Nil(t, err)

	t.Log("sending transaction: ", tx.ID())

	ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
	err = client.SendTransaction(ctx, *tx)
	cancel()
	assert.Nil(t, err)

	// wait for the transaction to be included in a collection
	suite.AwaitTransactionIncluded(convert.IDFromSDK(tx.ID()))
	suite.net.Stop()

	state := suite.ClusterStateFor(col1.Config.NodeID)

	// the transaction should be included in exactly one collection
	checker := unittest.NewClusterStateChecker(state)
	checker.
		ExpectContainsTx(convert.IDFromSDK(tx.ID())).
		ExpectTxCount(1).
		Assert(t)
}

// Test sending a single valid transaction to the responsible cluster in a
// multi-cluster configuration
//
// The transaction should not be routed and should be included in exactly one
// collection in only the responsible cluster.
func (suite *CollectorSuite) TestTxIngressMultiCluster_CorrectCluster() {
	t := suite.T()

	// NOTE: we use 3-node clusters so that proposal messages are sent 1-K
	// as 1-1 messages are not picked up by the ghost node.
	const (
		nNodes    uint = 6
		nClusters uint = 2
	)

	suite.SetupTest("col_txingress_multicluster_correctcluster", nNodes, nClusters)

	clusters := suite.Clusters()

	// pick a cluster to target
	targetCluster := clusters.ByIndex(0)
	targetNode := suite.Collector(0, 0)

	// get a client pointing to the cluster member
	client, err := sdkclient.New(targetNode.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure())
	require.Nil(t, err)

	tx := suite.TxForCluster(targetCluster)

	// submit the transaction
	ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
	err = client.SendTransaction(ctx, *tx)
	cancel()
	assert.Nil(t, err)

	// wait for the transaction to be included in a collection
	suite.AwaitTransactionIncluded(convert.IDFromSDK(tx.ID()))
	suite.net.Stop()

	// ensure the transaction IS included in target cluster collection
	for _, id := range targetCluster.NodeIDs() {
		state := suite.ClusterStateFor(id)

		// the transaction should be included exactly once
		checker := unittest.NewClusterStateChecker(state)
		checker.
			ExpectContainsTx(convert.IDFromSDK(tx.ID())).
			ExpectTxCount(1).
			Assert(t)
	}

	// ensure the transaction IS NOT included in other cluster collections
	for _, cluster := range clusters.All() {
		// skip the target cluster
		if cluster.Fingerprint() == targetCluster.Fingerprint() {
			continue
		}

		for _, id := range cluster.NodeIDs() {
			state := suite.ClusterStateFor(id)

			// the transaction should not be included
			checker := unittest.NewClusterStateChecker(state)
			checker.
				ExpectOmitsTx(convert.IDFromSDK(tx.ID())).
				ExpectTxCount(0).
				Assert(t)
		}
	}
}

// Test sending a single valid transaction to a non-responsible cluster in a
// multi-cluster configuration
//
// The transaction should be routed to the responsible cluster and should be
// included in a collection in only the responsible cluster's state.
func (suite *CollectorSuite) TestTxIngressMultiCluster_OtherCluster() {
	t := suite.T()

	// NOTE: we use 3-node clusters so that proposal messages are sent 1-K
	// as 1-1 messages are not picked up by the ghost node.
	const (
		nNodes    uint = 6
		nClusters uint = 2
	)

	suite.SetupTest("col_txingress_multicluster_othercluster", nNodes, nClusters)

	clusters := suite.Clusters()

	// pick a cluster to target
	// this cluster is responsible for the transaction
	targetCluster := clusters.ByIndex(0)

	// pick 1 node from the other cluster to send the transaction to
	otherNode := suite.Collector(1, 0)

	// create clients pointing to each other node
	client, err := sdkclient.New(otherNode.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure())
	require.Nil(t, err)

	// create a transaction that will be routed to the target cluster
	tx := suite.TxForCluster(targetCluster)

	// submit the transaction to the other NON-TARGET cluster and retry
	// several times to give the mesh network a chance to form
	go func() {
		for i := 0; i < 10; i++ {
			select {
			case <-suite.ctx.Done():
				// exit when suite is finished
				return
			case <-time.After(100 * time.Millisecond << time.Duration(i)):
				// retry on an exponential backoff
			}

			ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
			_ = client.SendTransaction(ctx, *tx)
			cancel()
		}
	}()

	// wait for the transaction to be included in a collection
	suite.AwaitTransactionIncluded(convert.IDFromSDK(tx.ID()))
	suite.net.Stop()

	// ensure the transaction IS included in target cluster collection
	for _, id := range targetCluster.NodeIDs() {
		state := suite.ClusterStateFor(id)

		// the transaction should be included exactly once
		checker := unittest.NewClusterStateChecker(state)
		checker.
			ExpectContainsTx(convert.IDFromSDK(tx.ID())).
			ExpectTxCount(1).
			Assert(t)
	}

	// ensure the transaction IS NOT included in other cluster collections
	for _, cluster := range clusters.All() {
		// skip the target cluster
		if cluster.Fingerprint() == targetCluster.Fingerprint() {
			continue
		}

		for _, id := range cluster.NodeIDs() {
			state := suite.ClusterStateFor(id)

			// the transaction should not be included
			checker := unittest.NewClusterStateChecker(state)
			checker.
				ExpectOmitsTx(convert.IDFromSDK(tx.ID())).
				ExpectTxCount(0).
				Assert(t)
		}
	}
}
