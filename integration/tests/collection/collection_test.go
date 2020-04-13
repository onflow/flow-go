package collection

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	clusterstate "github.com/dapperlabs/flow-go/state/cluster/badger"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const (
	// the timeout for individual actions (eg. send a transaction)
	defaultTimeout = 10 * time.Second
	// the period we wait to give consensus/routing time to complete
	waitTime = 10 * time.Second
)

// default set of non-collection nodes
func defaultOtherNodes() []testnet.NodeConfig {
	var (
		conNode1 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.ErrorLevel))
		conNode2 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.ErrorLevel))
		conNode3 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.ErrorLevel))
		exeNode  = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.ErrorLevel))
		verNode  = testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.ErrorLevel))
	)

	return []testnet.NodeConfig{conNode1, conNode2, conNode3, exeNode, verNode}
}

// Test sending various invalid transactions to a single-cluster configuration.
// The transactions should be rejected by the collection node and not included
// in any collection.
func TestTransactionIngress_InvalidTransaction(t *testing.T) {
	var (
		colNode1 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(1))
		colNode2 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(2))
		colNode3 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(3))
	)

	nodes := append([]testnet.NodeConfig{colNode1, colNode2, colNode3}, defaultOtherNodes()...)
	conf := testnet.NewNetworkConfig(nodes)

	net, err := testnet.PrepareFlowNetwork(t, "col_invalid_txns", conf)
	require.Nil(t, err)

	ctx := context.Background()

	net.Start(ctx)
	defer net.Cleanup()

	// we will test against COL1
	colContainer1, ok := net.ContainerByID(colNode1.Identifier)
	assert.True(t, ok)

	client, err := colContainer1.Client(testnet.ColNodeAPIPort)
	require.Nil(t, err)

	t.Run("missing reference block hash", func(t *testing.T) {
		txDSL := unittest.TransactionDSLFixture()
		malformed := unittest.TransactionBodyFixture(unittest.WithTransactionDSL(txDSL))
		malformed.ReferenceBlockID = flow.ZeroID

		expected := ingest.ErrIncompleteTransaction{Missing: malformed.MissingFields()}

		ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
		err := client.SignAndSendTransaction(ctx, malformed)
		unittest.AssertErrSubstringMatch(t, expected, err)
	})

	t.Run("missing script", func(t *testing.T) {
		malformed := unittest.TransactionBodyFixture()
		malformed.Script = nil

		expected := ingest.ErrIncompleteTransaction{Missing: malformed.MissingFields()}

		ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
		err := client.SignAndSendTransaction(ctx, malformed)
		unittest.AssertErrSubstringMatch(t, expected, err)
	})

	t.Run("unparseable script", func(t *testing.T) {
		// TODO script parsing not implemented
		t.Skip()
	})
	t.Run("invalid signature", func(t *testing.T) {
		// TODO signature validation not implemented
		t.Skip()
	})
	t.Run("invalid nonce", func(t *testing.T) {
		// TODO nonce validation not implemented
		t.Skip()
	})
	t.Run("insufficient payer balance", func(t *testing.T) {
		// TODO balance checking not implemented
		t.Skip()
	})
	t.Run("expired transaction", func(t *testing.T) {
		// TODO blocked by https://github.com/dapperlabs/flow-go/issues/3005
		t.Skip()
	})
	t.Run("non-existent reference block ID", func(t *testing.T) {
		// TODO blocked by https://github.com/dapperlabs/flow-go/issues/3005
		t.Skip()
	})
}

// Test sending a single valid transaction to a single cluster.
// The transaction should be included in a collection.
func TestTransactionIngress_SingleCluster(t *testing.T) {

	var (
		colNode1 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(1))
		colNode2 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(2))
		colNode3 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(3))
	)

	nodes := append([]testnet.NodeConfig{colNode1, colNode2, colNode3}, defaultOtherNodes()...)
	conf := testnet.NewNetworkConfig(nodes)

	net, err := testnet.PrepareFlowNetwork(t, "col_single_cluster", conf)
	require.Nil(t, err)

	ctx := context.Background()

	net.Start(ctx)
	defer net.Cleanup()

	// we will test against COL1
	colContainer1, ok := net.ContainerByID(colNode1.Identifier)
	assert.True(t, ok)

	client, err := colContainer1.Client(testnet.ColNodeAPIPort)
	require.Nil(t, err)

	tx := unittest.TransactionBodyFixture()
	tx, err = client.SignTransaction(tx)
	assert.Nil(t, err)
	t.Log("sending transaction: ", tx.ID())

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	err = client.SendTransaction(ctx, tx)
	assert.Nil(t, err)

	// wait for consensus to complete
	//TODO we should listen for collection guarantees instead, but this is blocked
	// ref: https://github.com/dapperlabs/flow-go/issues/3021
	time.Sleep(waitTime)

	err = net.Stop()
	assert.Nil(t, err)

	identities := net.Identities()

	chainID := protocol.ChainIDForCluster(identities.Filter(filter.HasRole(flow.RoleCollection)))

	// get database for COL1
	db, err := colContainer1.DB()
	require.Nil(t, err)

	state, err := clusterstate.NewState(db, chainID)
	assert.Nil(t, err)

	// the transaction should be included in exactly one collection
	checker := unittest.NewClusterStateChecker(state)
	checker.
		ExpectContainsTx(tx.ID()).
		ExpectTxCount(1).
		Assert(t)
}

// Test sending a single valid transaction to multi-cluster configuration.
//
// We want to ensure that the transaction is routed to the appropriate cluster
// and included in a collection in the appropriate cluster.
func TestTransactionIngress_MultiCluster(t *testing.T) {

	var (
		colNode1 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(1))
		colNode2 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(2))
		colNode3 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(3))
		colNode4 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(4))
		colNode5 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(5))
		colNode6 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(6))
	)

	const nClusters uint = 3

	nodes := append(
		[]testnet.NodeConfig{colNode1, colNode2, colNode3, colNode4, colNode5, colNode6},
		defaultOtherNodes()...,
	)
	conf := testnet.NewNetworkConfig(nodes, testnet.WithClusters(nClusters))

	net, err := testnet.PrepareFlowNetwork(t, "col_multi_cluster", conf)
	require.Nil(t, err)

	ctx := context.Background()

	net.Start(ctx)
	defer net.Cleanup()

	clusters := protocol.Clusters(nClusters, net.Identities())

	// Send the transaction to the cluster that is responsible for it. It
	// should be included in a collection in only that cluster.
	t.Run("send tx to responsible cluster", func(t *testing.T) {

		// pick a cluster to target
		targetCluster := clusters.ByIndex(0)
		targetIdentity, ok := targetCluster.ByIndex(0)
		require.True(t, ok)

		// pick a member of the cluster
		targetNode, ok := net.ContainerByID(targetIdentity.NodeID)
		require.True(t, ok)

		// get a client pointing to the cluster member
		client, err := targetNode.Client(testnet.ColNodeAPIPort)
		require.Nil(t, err)

		// create a transaction
		tx := unittest.TransactionBodyFixture()
		tx, err = client.SignTransaction(tx)
		assert.Nil(t, err)

		// submit the transaction
		ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
		err = client.SendTransaction(ctx, tx)
		assert.Nil(t, err)

		// wait for consensus to complete
		time.Sleep(waitTime)

		err = net.Pause()
		require.Nil(t, err)
		//defer func() {
		//	err = net.Unpause()
		//	require.Nil(t, err)
		//}()

		chainID := protocol.ChainIDForCluster(targetCluster)

		// get database for target node
		db, err := targetNode.DB()
		require.Nil(t, err)

		state, err := clusterstate.NewState(db, chainID)
		require.Nil(t, err)

		// the transaction should be included in exactly one collection
		checker := unittest.NewClusterStateChecker(state)
		checker.
			ExpectContainsTx(tx.ID()).
			ExpectTxCount(1).
			Assert(t)
	})

	t.Run("send tx to other cluster", func(t *testing.T) {})

	t.Run("send tx to multiple other clusters", func(t *testing.T) {})
}
